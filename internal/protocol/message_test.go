package protocol

import (
	"testing"
	"time"
)

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	now := time.Unix(0, 1759999999999000000)
	in := MessageEnvelope{
		RoomID:      42,
		SenderID:    7,
		ClientMsgID: "msg-abc",
		Type:        1,
		Content:     "hello protobuf",
		CreatedAt:   now,
	}

	data, err := Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty protobuf payload")
	}

	out, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.RoomID != in.RoomID || out.SenderID != in.SenderID || out.ClientMsgID != in.ClientMsgID {
		t.Fatalf("unexpected envelope: %+v", out)
	}
	if out.Type != in.Type || out.Content != in.Content {
		t.Fatalf("unexpected content/type: %+v", out)
	}
	if out.CreatedAt.UnixNano() != now.UnixNano() {
		t.Fatalf("created_at = %v, want %v", out.CreatedAt, now)
	}
}

func TestUnmarshalLegacyJSON(t *testing.T) {
	data := []byte(`{"room_id":"42","sender_id":7,"content":"legacy","client_msg_id":"msg-old","timestamp":1759999999999000000}`)
	out, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("legacy unmarshal failed: %v", err)
	}
	if out.RoomID != 42 || out.SenderID != 7 || out.Content != "legacy" {
		t.Fatalf("unexpected legacy envelope: %+v", out)
	}
	if out.ClientMsgID != "msg-old" || out.Type != 1 {
		t.Fatalf("unexpected legacy identity: %+v", out)
	}
	if out.CreatedAt.UnixNano() != 1759999999999000000 {
		t.Fatalf("legacy created_at = %v", out.CreatedAt)
	}
}

func TestUnmarshalInvalidPayload(t *testing.T) {
	if _, err := Unmarshal(nil); err == nil {
		t.Fatal("expected error for nil payload")
	}
	if _, err := Unmarshal([]byte{0xff, 0xff, 0xff}); err == nil {
		t.Fatal("expected error for invalid payload")
	}
}
