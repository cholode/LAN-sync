package producer

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

type MessageClient struct {
	writer *kafka.Writer
}

func NewMessageClient(brokers []string, topic string) *MessageClient {
	return &MessageClient{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.Hash{},
			Async:    true,
		},
	}
}

func (c *MessageClient) HandleIncomingMessage(ctx context.Context, roomID string, senderID int, content string) {
	payload, _ := json.Marshal(map[string]interface{}{
		"room_id":   roomID,
		"sender_id": senderID,
		"content":   content,
		"timestamp": time.Now().UnixNano(),
	})

	err := c.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(roomID),
		Value: payload,
	})

	if err != nil {
		log.Printf("Kafka 物理投递失败：%v", err)
	}
}

func (c *MessageClient) Close() error {
	return c.writer.Close()
}
