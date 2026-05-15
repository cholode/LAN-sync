package producer

import (
	"context"
	"encoding/json"
	"fmt"
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

func (c *MessageClient) HandleIncomingMessage(ctx context.Context, roomID string, senderID int, content string, clientMsgID string) error {
	payload, err := json.Marshal(map[string]interface{}{
		"room_id":       roomID,
		"sender_id":     senderID,
		"content":       content,
		"client_msg_id": clientMsgID,
		"timestamp":     time.Now().UnixNano(),
	})

	if err != nil {
		return fmt.Errorf("消息负载序列化失败: %w", err)
	}

	// 2. 物理投递
	err = c.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(roomID), // 物理保证：同群同分区！
		Value: payload,
	})

	if err != nil {
		log.Printf("[中间件告警] Kafka 投递失败：%v", err)
		// 必须将错误抛给调用方 (ReadPump)，让网关决定是重试还是向客户端返回发送失败
		return fmt.Errorf("kafka 写入失败: %w", err)
	}

	return nil
}

func (c *MessageClient) Close() error {
	return c.writer.Close()
}
