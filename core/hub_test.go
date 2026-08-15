package core

import (
	"context"
	"encoding/json"
	"lan-im-go/models"
	"sync"
	"testing"
	"time"
)

// TestHub_SubscribeAndUnsubscribe 测试客户端订阅和退订房间。
func TestHub_SubscribeAndUnsubscribe(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	client := &Client{
		Hub:    hub,
		UserID: 1,
		Send:   make(chan []byte, 256),
	}

	hub.Subscribe <- &Subscription{
		Client:  client,
		RoomIDs: []int64{100, 200},
	}

	time.Sleep(20 * time.Millisecond)

	if _, ok := hub.users[1]; !ok {
		t.Fatal("user not found in hub after subscribe")
	}
	if _, ok := hub.rooms[100]; !ok {
		t.Fatal("room 100 not found after subscribe")
	}
	if _, ok := hub.rooms[200]; !ok {
		t.Fatal("room 200 not found after subscribe")
	}

	hub.Unsubscribe <- &Subscription{
		Client:  client,
		RoomIDs: nil,
	}

	time.Sleep(20 * time.Millisecond)

	if _, ok := hub.users[1]; ok {
		t.Fatal("user still in hub after unsubscribe")
	}
	if _, ok := hub.rooms[100]; ok {
		t.Fatal("room 100 still exists after user left")
	}

	// 退订后发送通道应被关闭。
	select {
	case _, ok := <-client.Send:
		if ok {
			t.Fatal("send channel should be closed after unsubscribe")
		}
	default:
	}
}

// TestHub_ForwardMessage 测试消息转发给单个客户端。
func TestHub_ForwardMessage(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	client := &Client{
		Hub:    hub,
		UserID: 1,
		Send:   make(chan []byte, 256),
	}

	hub.Subscribe <- &Subscription{
		Client:  client,
		RoomIDs: []int64{100},
	}

	time.Sleep(20 * time.Millisecond)

	msg := &models.Message{
		RoomID:      100,
		SenderID:    2,
		Content:     "hello",
		ClientMsgID: "msg-001",
		Type:        1,
	}

	hub.ForwardMessage <- msg

	select {
	case raw := <-client.Send:
		var received models.Message
		if err := json.Unmarshal(raw, &received); err != nil {
			t.Fatalf("failed to unmarshal forwarded message: %v", err)
		}
		if received.Content != "hello" {
			t.Errorf("content = %s, want hello", received.Content)
		}
		if received.RoomID != 100 {
			t.Errorf("roomID = %d, want 100", received.RoomID)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for forwarded message")
	}
}

// TestHub_ForwardMessageToMultipleClients 测试消息广播给多个客户端。
func TestHub_ForwardMessageToMultipleClients(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	var clients []*Client
	for i := 1; i <= 3; i++ {
		client := &Client{
			Hub:    hub,
			UserID: int64(i),
			Send:   make(chan []byte, 256),
		}
		clients = append(clients, client)
		hub.Subscribe <- &Subscription{
			Client:  client,
			RoomIDs: []int64{100},
		}
	}

	time.Sleep(30 * time.Millisecond)

	msg := &models.Message{
		RoomID:      100,
		SenderID:    99,
		Content:     "broadcast",
		ClientMsgID: "msg-002",
		Type:        1,
	}

	hub.ForwardMessage <- msg

	received := 0
	deadline := time.After(300 * time.Millisecond)
	for _, client := range clients {
		select {
		case <-client.Send:
			received++
		case <-deadline:
			t.Fatalf("timed out; received %d/3 messages", received)
		}
	}

	if received != 3 {
		t.Errorf("received = %d, want 3", received)
	}
}

// TestHub_RoomAction_JoinAndLeave 测试房间动作中的加入与离开。
func TestHub_RoomAction_JoinAndLeave(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	client := &Client{
		Hub:    hub,
		UserID: 1,
		Send:   make(chan []byte, 256),
	}

	hub.Subscribe <- &Subscription{
		Client:  client,
		RoomIDs: []int64{},
	}

	time.Sleep(20 * time.Millisecond)

	hub.RoomActionChan <- &RoomAction{
		UserID: 1,
		RoomID: 300,
		Action: "join",
	}

	time.Sleep(20 * time.Millisecond)

	if _, ok := hub.rooms[300]; !ok {
		t.Fatal("room 300 not found after join")
	}
	if !hub.rooms[300][client] {
		t.Fatal("client not in room 300 after join")
	}

	hub.RoomActionChan <- &RoomAction{
		UserID: 1,
		RoomID: 300,
		Action: "leave",
	}

	time.Sleep(20 * time.Millisecond)

	if hub.rooms[300] != nil && hub.rooms[300][client] {
		t.Fatal("client still in room 300 after leave")
	}
}

// TestHub_RoomAction_Disband 测试房间解散动作。
func TestHub_RoomAction_Disband(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	client := &Client{
		Hub:    hub,
		UserID: 1,
		Send:   make(chan []byte, 256),
	}

	hub.Subscribe <- &Subscription{
		Client:  client,
		RoomIDs: []int64{400},
	}

	time.Sleep(20 * time.Millisecond)

	hub.RoomActionChan <- &RoomAction{
		UserID: 1,
		RoomID: 400,
		Action: "disband",
	}

	time.Sleep(20 * time.Millisecond)

	if _, ok := hub.rooms[400]; ok {
		t.Fatal("room 400 still exists after disband")
	}
}

// TestHub_ConcurrentSubscribe 测试多个客户端并发订阅。
func TestHub_ConcurrentSubscribe(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	var wg sync.WaitGroup
	const numClients = 50

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			client := &Client{
				Hub:    hub,
				UserID: int64(id),
				Send:   make(chan []byte, 256),
			}
			hub.Subscribe <- &Subscription{
				Client:  client,
				RoomIDs: []int64{500},
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	if len(hub.users) != numClients {
		t.Errorf("users = %d, want %d", len(hub.users), numClients)
	}
	if len(hub.rooms[500]) != numClients {
		t.Errorf("room 500 clients = %d, want %d", len(hub.rooms[500]), numClients)
	}
}

// TestHub_Shutdown 测试 Hub 收到上下文取消后能正常退出。
func TestHub_Shutdown(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Hub 已正常退出。
	case <-time.After(200 * time.Millisecond):
		t.Fatal("hub did not shut down in time")
	}
}
