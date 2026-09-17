package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHeartbeatEnabled(t *testing.T) {
	t.Setenv("WS_HEARTBEAT_ENABLED", "false")
	if heartbeatEnabled() {
		t.Fatal("heartbeat should be disabled")
	}
	t.Setenv("WS_HEARTBEAT_ENABLED", "true")
	if !heartbeatEnabled() {
		t.Fatal("heartbeat should be enabled")
	}
}

func TestWritePumpAggregatesMessagesWithinWindow(t *testing.T) {
	serverConn := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		serverConn <- conn
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	peer, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer peer.Close()

	client := &Client{Conn: <-serverConn, Send: make(chan OutboundMessage, 16)}
	done := make(chan struct{})
	go func() {
		client.WritePump()
		close(done)
	}()

	started := time.Now()
	client.Send <- OutboundMessage{Payload: []byte(`{"sequence":1}`), GatewayArrivedAt: time.Now()}
	time.Sleep(5 * time.Millisecond)
	client.Send <- OutboundMessage{Payload: []byte(`{"sequence":2}`), GatewayArrivedAt: time.Now()}
	client.Send <- OutboundMessage{Payload: []byte(`{"sequence":3}`), GatewayArrivedAt: time.Now()}

	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	messageType, payload, err := peer.ReadMessage()
	if err != nil {
		t.Fatalf("read aggregated message: %v", err)
	}
	if messageType != websocket.TextMessage {
		t.Fatalf("message type = %d, want text", messageType)
	}
	if got, want := string(payload), "{\"sequence\":1}\n{\"sequence\":2}\n{\"sequence\":3}"; got != want {
		t.Fatalf("payload = %q, want %q", got, want)
	}
	if elapsed := time.Since(started); elapsed < writeBatchWindow-5*time.Millisecond {
		t.Fatalf("batch flushed too early after %v", elapsed)
	}

	close(client.Send)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WritePump did not stop after send queue closed")
	}
}

func TestWritePumpFlushesImmediatelyAtMessageLimit(t *testing.T) {
	serverConn := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		serverConn <- conn
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	peer, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer peer.Close()

	client := &Client{Conn: <-serverConn, Send: make(chan OutboundMessage, writeBatchMaxMessages)}
	done := make(chan struct{})
	go func() {
		client.WritePump()
		close(done)
	}()

	started := time.Now()
	for i := 0; i < writeBatchMaxMessages; i++ {
		client.Send <- OutboundMessage{Payload: []byte(`{}`), GatewayArrivedAt: time.Now()}
	}
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, payload, err := peer.ReadMessage()
	if err != nil {
		t.Fatalf("read size-limited batch: %v", err)
	}
	if got := len(strings.Split(string(payload), "\n")); got != writeBatchMaxMessages {
		t.Fatalf("batch message count = %d, want %d", got, writeBatchMaxMessages)
	}
	if elapsed := time.Since(started); elapsed >= writeBatchWindow {
		t.Fatalf("message limit did not flush early: %v", elapsed)
	}

	close(client.Send)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WritePump did not stop after send queue closed")
	}
}
