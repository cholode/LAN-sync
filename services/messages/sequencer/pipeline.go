package sequencer

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	protocol "lan-im-go/contracts/events"
)

type Reader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
}

type WriteFunc func(context.Context, kafka.Message) error

// Retry 在当前消息成功前不推进下一条，输出结果未知时仍重发相同编号。
func Retry(ctx context.Context, action func() error) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := action(); err == nil {
			return nil
		} else {
			log.Printf("消息处理失败，保留当前消息并重试: %v", err)
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func Run(ctx context.Context, reader Reader, write WriteFunc, state *Sequencer) error {
	for {
		record, err := reader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		input, err := protocol.Unmarshal(record.Value)
		if err != nil {
			return fmt.Errorf("待处理消息解析失败 offset=%d: %w", record.Offset, err)
		}
		message, err := state.Assign(input)
		if err != nil {
			// 非法输入不改变编号状态，明确记录后跳过，避免单条冲突阻断所有群。
			log.Printf("拒绝待处理消息 offset=%d: %v", record.Offset, err)
		} else {
			value, err := protocol.Marshal(message)
			if err != nil {
				return err
			}
			output := kafka.Message{Key: []byte(fmt.Sprint(message.RoomID)), Value: value}
			if err := Retry(ctx, func() error { return write(ctx, output) }); err != nil {
				return err
			}
		}
		if err := Retry(ctx, func() error { return reader.CommitMessages(ctx, record) }); err != nil {
			return err
		}
	}
}

// Relay 独立消费正式消息，允许重放；前端和其他消费者按业务 ID 去重。
func Relay(ctx context.Context, reader Reader, publish func(context.Context, protocol.MessageEnvelope, []byte) error) error {
	for {
		record, err := reader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		message, err := protocol.Unmarshal(record.Value)
		if err != nil {
			return err
		}
		if message.MessageID <= 0 || message.RoomSeq <= 0 {
			return fmt.Errorf("正式消息缺少编号 offset=%d", record.Offset)
		}
		if err := Retry(ctx, func() error { return publish(ctx, message, record.Value) }); err != nil {
			return err
		}
		if err := Retry(ctx, func() error { return reader.CommitMessages(ctx, record) }); err != nil {
			return err
		}
	}
}

// EnsureTopics 显式创建单分区主题，避免旧归档器只读取分区 0 而遗漏消息。
func EnsureTopics(ctx context.Context, brokers []string, topics ...string) error {
	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	controller, err := conn.Controller()
	if err != nil {
		return err
	}
	admin, err := kafka.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return err
	}
	defer admin.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = admin.SetDeadline(deadline)
	}
	for _, topic := range topics {
		if err := admin.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1}); err != nil {
			return err
		}
		// 创建响应返回后元数据可能尚未传播，等待主题真正可读再启动消费者。
		var partitions []kafka.Partition
		if err := Retry(ctx, func() error {
			var readErr error
			partitions, readErr = conn.ReadPartitions(topic)
			return readErr
		}); err != nil {
			return err
		}
		if len(partitions) != 1 {
			return fmt.Errorf("当前版本要求主题 %s 恰好一个分区，实际 %d", topic, len(partitions))
		}
	}
	return nil
}
