package producer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"

	"lan-im-go/contracts/events"
	"lan-im-go/pkg"
	"lan-im-go/shared/observability/metrics"
)

type MessageClient struct {
	writer *kafka.Writer
	topic  string
}

func NewMessageClient(brokers []string, topic string) *MessageClient {
	return &MessageClient{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			Async:        false,
			RequiredAcks: kafka.RequireAll,
			BatchTimeout: 5 * time.Millisecond,
		},
		topic: topic,
	}
}

func (c *MessageClient) HandleIncomingMessage(ctx context.Context, roomID string, senderID int, content string, clientMsgID string) error {
	roomIDInt, err := strconv.ParseInt(roomID, 10, 64)
	if err != nil {
		return fmt.Errorf("非法 room_id: %w", err)
	}

	payload, err := protocol.Marshal(protocol.MessageEnvelope{
		RoomID:      roomIDInt,
		SenderID:    int64(senderID),
		ClientMsgID: clientMsgID,
		Type:        1,
		Content:     content,
		CreatedAt:   time.Now(),
	})
	if err != nil {
		return fmt.Errorf("消息负载序列化失败: %w", err)
	}

	start := time.Now()
	err = c.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(roomID),
		Value: payload,
	})
	metrics.ObserveKafkaProduce(c.topic, start, err)
	if err != nil {
		pkg.Infof("[中间件告警] Kafka 投递失败：%v", err)
		return fmt.Errorf("kafka 写入失败: %w", err)
	}

	// 广播由正式消息消费者负责，禁止未编号消息绕过编号器。
	return nil
}

func (c *MessageClient) Close() error {
	return c.writer.Close()
}
