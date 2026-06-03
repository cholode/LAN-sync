package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"
)

type MessageClient struct {
	writer      *kafka.Writer
	redisClient *redis.Client
}

func NewMessageClient(brokers []string, topic string, redisClient *redis.Client) *MessageClient {
	return &MessageClient{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.Hash{},
			Async:    true,
		},
		redisClient: redisClient,
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

	err = c.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(roomID),
		Value: payload,
	})

	if err != nil {
		log.Printf("[中间件告警] Kafka 投递失败：%v", err)
		return fmt.Errorf("kafka 写入失败: %w", err)
	}

	redisChannel := "im:broadcast:room:" + roomID
	if pubErr := c.redisClient.Publish(ctx, redisChannel, string(payload)).Err(); pubErr != nil {
		log.Printf("[中间件告警] Redis 广播发布失败（消息仍已落盘Kafka）: %v", pubErr)
	}

	return nil
}

func (c *MessageClient) Close() error {
	return c.writer.Close()
}