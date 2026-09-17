package producer

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"

	protocol "lan-im-go/contracts/events"
	"lan-im-go/shared/observability/logger"
	"lan-im-go/shared/observability/metrics"
)

var (
	ErrProducerClosed    = errors.New("Kafka 生产者已关闭")
	ErrProducerQueueFull = errors.New("Kafka 生产队列已满")
)

type BatchOptions struct {
	MaxMessages   int
	MaxBytes      int
	MaxWait       time.Duration
	QueueCapacity int
	WriteTimeout  time.Duration
}

type messageWriter interface {
	WriteMessages(context.Context, ...kafka.Message) error
	Close() error
}

type produceRequest struct {
	message kafka.Message
	started time.Time
	result  chan error
}

type MessageClient struct {
	writer messageWriter
	topic  string
	opts   BatchOptions
	queue  chan *produceRequest
	stop   chan struct{}
	done   chan struct{}

	stateMu   sync.RWMutex
	closed    bool
	closeOnce sync.Once
	closeErr  error
}

func NewMessageClient(brokers []string, topic string, options ...BatchOptions) *MessageClient {
	var opts BatchOptions
	if len(options) > 0 {
		opts = options[0]
	}
	opts = normalizedOptions(opts)
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		Async:        false,
		RequiredAcks: kafka.RequireAll,
		BatchSize:    opts.MaxMessages,
		BatchBytes:   int64(opts.MaxBytes),
		BatchTimeout: time.Millisecond,
	}
	return newMessageClient(writer, topic, opts)
}

func normalizedOptions(opts BatchOptions) BatchOptions {
	if opts.MaxMessages <= 0 {
		opts.MaxMessages = 1000
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 1 << 20
	}
	if opts.MaxWait <= 0 {
		opts.MaxWait = 5 * time.Millisecond
	}
	if opts.QueueCapacity <= 0 {
		opts.QueueCapacity = 20000
	}
	if opts.WriteTimeout <= 0 {
		opts.WriteTimeout = 10 * time.Second
	}
	return opts
}

func newMessageClient(writer messageWriter, topic string, opts BatchOptions) *MessageClient {
	opts = normalizedOptions(opts)
	client := &MessageClient{
		writer: writer,
		topic:  topic,
		opts:   opts,
		queue:  make(chan *produceRequest, opts.QueueCapacity),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go client.run()
	return client
}

func (c *MessageClient) HandleIncomingMessage(ctx context.Context, roomID string, senderID int, content string, clientMsgID string) error {
	return c.HandleIncomingMessageAt(ctx, roomID, senderID, content, clientMsgID, time.Now())
}

// HandleIncomingMessageAt preserves the time at which the message entered the
// Gateway so downstream delivery latency excludes the client network path.
func (c *MessageClient) HandleIncomingMessageAt(ctx context.Context, roomID string, senderID int, content string, clientMsgID string, gatewayArrivedAt time.Time) error {
	roomIDInt, err := strconv.ParseInt(roomID, 10, 64)
	if err != nil {
		return fmt.Errorf("非法 room_id: %w", err)
	}
	if gatewayArrivedAt.IsZero() {
		gatewayArrivedAt = time.Now()
	}
	payload, err := protocol.Marshal(protocol.MessageEnvelope{
		RoomID: roomIDInt, SenderID: int64(senderID), ClientMsgID: clientMsgID,
		Type: 1, Content: content, CreatedAt: gatewayArrivedAt,
	})
	if err != nil {
		return fmt.Errorf("消息负载序列化失败: %w", err)
	}
	request := &produceRequest{
		message: kafka.Message{Key: []byte(roomID), Value: payload},
		started: time.Now(), result: make(chan error, 1),
	}
	if len(request.message.Key)+len(request.message.Value) > c.opts.MaxBytes {
		return fmt.Errorf("消息超过 Kafka 批次字节上限")
	}

	c.stateMu.RLock()
	if c.closed {
		c.stateMu.RUnlock()
		return ErrProducerClosed
	}
	select {
	case c.queue <- request:
		metrics.SetKafkaProducerQueueDepth(c.topic, len(c.queue))
		c.stateMu.RUnlock()
	case <-ctx.Done():
		c.stateMu.RUnlock()
		return ctx.Err()
	default:
		c.stateMu.RUnlock()
		metrics.ObserveKafkaProducerQueueRejection(c.topic, "full")
		return ErrProducerQueueFull
	}

	select {
	case err := <-request.result:
		if err != nil {
			return fmt.Errorf("kafka 批量写入失败: %w", err)
		}
		return nil
	case <-ctx.Done():
		// 已入队消息仍可能成功写入；客户端用同一 ClientMsgID 重试可安全去重。
		return ctx.Err()
	}
}

func (c *MessageClient) run() {
	defer close(c.done)
	var pending *produceRequest
	stopping := false
	for {
		var first *produceRequest
		if pending != nil {
			first, pending = pending, nil
		} else if stopping {
			select {
			case first = <-c.queue:
			default:
				return
			}
		} else {
			select {
			case first = <-c.queue:
			case <-c.stop:
				stopping = true
				continue
			}
		}

		batch := make([]*produceRequest, 1, min(c.opts.MaxMessages, 64))
		batch[0] = first
		bytes := len(first.message.Key) + len(first.message.Value)
		timer := time.NewTimer(c.opts.MaxWait)
	collect:
		for len(batch) < c.opts.MaxMessages && bytes < c.opts.MaxBytes {
			if stopping {
				select {
				case next := <-c.queue:
					size := len(next.message.Key) + len(next.message.Value)
					if bytes+size > c.opts.MaxBytes {
						pending = next
						break collect
					}
					batch = append(batch, next)
					bytes += size
				default:
					break collect
				}
				continue
			}
			select {
			case next := <-c.queue:
				size := len(next.message.Key) + len(next.message.Value)
				if bytes+size > c.opts.MaxBytes {
					pending = next
					break collect
				}
				batch = append(batch, next)
				bytes += size
			case <-timer.C:
				break collect
			case <-c.stop:
				stopping = true
			}
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}

		messages := make([]kafka.Message, len(batch))
		for i, request := range batch {
			messages[i] = request.message
		}
		metrics.SetKafkaProducerQueueDepth(c.topic, len(c.queue))
		metrics.ObserveKafkaProducerBatch(c.topic, len(batch))
		writeCtx, cancel := context.WithTimeout(context.Background(), c.opts.WriteTimeout)
		err := c.writer.WriteMessages(writeCtx, messages...)
		cancel()
		if err != nil {
			logger.Infof("[中间件告警] Kafka 批量投递失败 count=%d: %v", len(batch), err)
		}
		for _, request := range batch {
			metrics.ObserveKafkaProduce(c.topic, request.started, err)
			request.result <- err
		}
	}
}

func (c *MessageClient) Close() error {
	c.closeOnce.Do(func() {
		c.stateMu.Lock()
		c.closed = true
		close(c.stop)
		c.stateMu.Unlock()
		<-c.done
		c.closeErr = c.writer.Close()
	})
	return c.closeErr
}
