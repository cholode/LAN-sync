package core

import (
	"context"
	"encoding/json"
	"fmt"
	"lan-im-go/models"
	"sync"
	"testing"
	"time"
)

func startHub(t *testing.T) (*Hub, context.Context, context.CancelFunc) {
	t.Helper()
	hub := NewHubWithShards(4)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("hub did not stop")
		}
	})
	return hub, ctx, cancel
}

func clientFor(hub *Hub, userID int64) *Client {
	return &Client{
		Hub:    hub,
		UserID: userID,
		Send:   make(chan []byte, 256),
	}
}

func roomClients(t *testing.T, hub *Hub, roomID int64) map[*Client]bool {
	t.Helper()
	shard := hub.shardForRoom(roomID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.rooms[roomID]
}

func TestHub_SubscribeAndUnsubscribe(t *testing.T) {
	hub, _, _ := startHub(t)
	client := clientFor(hub, 1)

	hub.Register(client, []int64{100, 200})

	if _, ok := hub.shardForUser(1).users[1]; !ok {
		t.Fatal("user not found in hub after subscribe")
	}
	if _, ok := hub.shardForRoom(100).rooms[100]; !ok {
		t.Fatal("room 100 not found after subscribe")
	}
	if _, ok := hub.shardForRoom(200).rooms[200]; !ok {
		t.Fatal("room 200 not found after subscribe")
	}

	hub.Unregister(client)

	if _, ok := hub.shardForUser(1).users[1]; ok {
		t.Fatal("user still in hub after unsubscribe")
	}
	if _, ok := hub.shardForRoom(100).rooms[100]; ok {
		t.Fatal("room 100 still exists after user left")
	}

	select {
	case _, ok := <-client.Send:
		if ok {
			t.Fatal("send channel should be closed after unsubscribe")
		}
	default:
	}
}

func TestHub_ForwardMessage(t *testing.T) {
	hub, _, _ := startHub(t)
	client := clientFor(hub, 1)

	hub.Register(client, []int64{100})

	msg := &models.Message{
		RoomID:      100,
		SenderID:    2,
		Content:     "hello",
		ClientMsgID: "msg-001",
		Type:        1,
	}

	hub.Publish(msg)

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
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for forwarded message")
	}
}

func TestHub_ForwardMessageToMultipleClients(t *testing.T) {
	hub, _, _ := startHub(t)

	var clients []*Client
	for i := 1; i <= 3; i++ {
		client := clientFor(hub, int64(i))
		clients = append(clients, client)
		hub.Register(client, []int64{100})
	}

	msg := &models.Message{
		RoomID:      100,
		SenderID:    99,
		Content:     "broadcast",
		ClientMsgID: "msg-002",
		Type:        1,
	}

	hub.Publish(msg)

	received := 0
	deadline := time.After(500 * time.Millisecond)
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

func TestHub_LargeRoomFanoutPreservesMessageOrder(t *testing.T) {
	hub, _, _ := startHub(t)
	hub.fanoutThreshold = 10
	hub.fanoutBatchSize = 20

	const clientCount = 128
	clients := make([]*Client, 0, clientCount)
	for i := 0; i < clientCount; i++ {
		client := clientFor(hub, int64(i+1))
		clients = append(clients, client)
		hub.Register(client, []int64{600})
	}

	for sequence := 1; sequence <= 2; sequence++ {
		hub.Publish(&models.Message{
			RoomID:      600,
			SenderID:    999,
			Content:     "fanout",
			ClientMsgID: fmt.Sprintf("msg-%d", sequence),
			Type:        1,
		})
	}

	deadline := time.After(2 * time.Second)
	for _, client := range clients {
		for sequence := 1; sequence <= 2; sequence++ {
			select {
			case raw := <-client.Send:
				var received models.Message
				if err := json.Unmarshal(raw, &received); err != nil {
					t.Fatalf("unmarshal fanout message: %v", err)
				}
				want := fmt.Sprintf("msg-%d", sequence)
				if received.ClientMsgID != want {
					t.Fatalf("client %d message order: got %s, want %s", client.UserID, received.ClientMsgID, want)
				}
			case <-deadline:
				t.Fatalf("timed out waiting for client %d message %d", client.UserID, sequence)
			}
		}
	}
}

func TestHub_RoomAction_JoinAndLeave(t *testing.T) {
	hub, _, _ := startHub(t)
	client := clientFor(hub, 1)
	hub.Register(client, nil)

	hub.JoinRoom(1, 300)

	if !roomClients(t, hub, 300)[client] {
		t.Fatal("client not in room 300 after join")
	}

	hub.LeaveRoom(1, 300)

	if roomClients(t, hub, 300) != nil && roomClients(t, hub, 300)[client] {
		t.Fatal("client still in room 300 after leave")
	}
}

func TestHub_RoomAction_Disband(t *testing.T) {
	hub, _, _ := startHub(t)
	client := clientFor(hub, 1)
	hub.Register(client, []int64{400})

	hub.DisbandRoom(400)

	if _, ok := hub.shardForRoom(400).rooms[400]; ok {
		t.Fatal("room 400 still exists after disband")
	}
}

func TestHub_ConcurrentSubscribe(t *testing.T) {
	hub, _, _ := startHub(t)

	var wg sync.WaitGroup
	const numClients = 50

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			client := clientFor(hub, int64(id))
			hub.Register(client, []int64{500})
		}(i)
	}

	wg.Wait()

	stats := hub.Stats()
	if stats.ClientCount != numClients {
		t.Errorf("users = %d, want %d", stats.ClientCount, numClients)
	}
	if clients := roomClients(t, hub, 500); len(clients) != numClients {
		t.Errorf("room 500 clients = %d, want %d", len(clients), numClients)
	}
}

func TestHub_Shutdown(t *testing.T) {
	hub := NewHubWithShards(2)
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
	case <-time.After(500 * time.Millisecond):
		t.Fatal("hub did not shut down in time")
	}
}
