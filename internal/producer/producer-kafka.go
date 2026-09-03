package producer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"

	"lan-im-go/internal/metrics"
	"lan-im-go/internal/protocol"
	"lan-im-go/pkg"
)

type MessageClient struct {
	writer      *kafka.Writer
	redisClient *redis.Client
	topic       string
}

func NewMessageClient(brokers []string, topic string, async bool, redisClient *redis.Client) *MessageClient {
	return &MessageClient{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.Hash{},
			Async:    async,
		},
		redisClient: redisClient,
		topic:       topic,
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

	redisChannel := "im:broadcast:room:" + roomID
	pubErr := c.redisClient.Publish(ctx, redisChannel, payload).Err()
	metrics.ObserveRedisPubSub(redisChannel, "publish", pubErr)
	if pubErr != nil {
		pkg.Infof("[中间件告警] Redis 广播发布失败（消息仍已落盘Kafka）: %v", pubErr)
	}

	return nil
}

func (c *MessageClient) Close() error {
	return c.writer.Close()
}
