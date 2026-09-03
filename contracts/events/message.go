package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/protobuf/proto"

	imv1 "lan-im-go/proto/im/v1"
)

type MessageEnvelope struct {
	RoomID      int64
	SenderID    int64
	ClientMsgID string
	Type        int8
	Content     string
	CreatedAt   time.Time
}

type legacyMessage struct {
	RoomID      string `json:"room_id"`
	SenderID    int64  `json:"sender_id"`
	Content     string `json:"content"`
	ClientMsgID string `json:"client_msg_id"`
	Timestamp   int64  `json:"timestamp"`
}

func Marshal(msg MessageEnvelope) ([]byte, error) {
	createdAtNS := msg.CreatedAt.UnixNano()
	if createdAtNS < 0 {
		createdAtNS = 0
	}

	return proto.Marshal(&imv1.ChatMessage{
		RoomId:      msg.RoomID,
		SenderId:    msg.SenderID,
		ClientMsgId: msg.ClientMsgID,
		Type:        int32(msg.Type),
		Content:     msg.Content,
		CreatedAtNs: createdAtNS,
	})
}

func Unmarshal(data []byte) (MessageEnvelope, error) {
	if len(data) == 0 {
		return MessageEnvelope{}, errors.New("empty message payload")
	}

	if msg, err := UnmarshalProto(data); err == nil {
		return msg, nil
	}

	if msg, err := UnmarshalLegacyJSON(data); err == nil {
		return msg, nil
	}

	return MessageEnvelope{}, errors.New("unsupported chat message payload")
}

func UnmarshalProto(data []byte) (MessageEnvelope, error) {
	var msg imv1.ChatMessage
	if err := proto.Unmarshal(data, &msg); err != nil {
		return MessageEnvelope{}, err
	}

	msgType := msg.GetType()
	if msgType == 0 {
		msgType = 1
	}

	msgEnvelope := MessageEnvelope{
		RoomID:      msg.GetRoomId(),
		SenderID:    msg.GetSenderId(),
		ClientMsgID: msg.GetClientMsgId(),
		Type:        int8(msgType),
		Content:     msg.GetContent(),
	}

	if ns := msg.GetCreatedAtNs(); ns > 0 {
		msgEnvelope.CreatedAt = time.Unix(0, ns)
	} else {
		msgEnvelope.CreatedAt = time.Now()
	}

	return msgEnvelope, nil
}

func UnmarshalLegacyJSON(data []byte) (MessageEnvelope, error) {
	var raw legacyMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return MessageEnvelope{}, err
	}

	if raw.RoomID == "" {
		return MessageEnvelope{}, errors.New("legacy message missing room_id")
	}

	roomID, err := strconv.ParseInt(raw.RoomID, 10, 64)
	if err != nil {
		return MessageEnvelope{}, fmt.Errorf("invalid room_id %q: %w", raw.RoomID, err)
	}

	createdAt := time.Now()
	if raw.Timestamp > 0 {
		createdAt = time.Unix(0, raw.Timestamp)
	}

	return MessageEnvelope{
		RoomID:      roomID,
		SenderID:    raw.SenderID,
		ClientMsgID: raw.ClientMsgID,
		Type:        1,
		Content:     raw.Content,
		CreatedAt:   createdAt,
	}, nil
}
