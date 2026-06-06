package archiver

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/segmentio/kafka-go"
	"lan-im-go/models"
	"lan-im-go/repository"
	"log"
	"strconv"
	"sync/atomic"
	"time"
)

type kafkaMessage struct {
	RoomID      string `json:"room_id"`
	SenderID    int64  `json:"sender_id"`
	Content     string `json:"content"`
	ClientMsgID string `json:"client_msg_id"`
}

var idSeq int64

func nextID() int64 {
	now := time.Now().UnixMilli()
	seq := atomic.AddInt64(&idSeq, 1) % 1000
	return now*1000 + seq
}

type Worker struct {
	reader *kafka.Reader
}

func NewWorker(brokers []string, topic string, groupID string) *Worker {
	return &Worker{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			GroupID:  groupID,
			Topic:    topic,
			MinBytes: 10e3,
			MaxBytes: 10e6,
			MaxWait:  500 * time.Millisecond,
		}),
	}
}

const (
	batchSize     = 1000
	flushInterval = 500 * time.Millisecond
)

func (w *Worker) Start(ctx context.Context) {
	defer w.reader.Close()

	log.Println("[Archiver] 稳态消费者已启动（微批模式: 1000条/500ms）")

	msgBatch := make([]*models.Message, 0, batchSize)
	// 记录本批次对应的最后一条 Kafka 消息，用于批量提交位移
	var lastMsg kafka.Message
	needCommit := false

	flush := func() {
		if len(msgBatch) == 0 {
			return
		}
		if err := repository.Message.SaveMessageBatch(msgBatch); err != nil {
			log.Printf("[Archiver] 批量写入失败，丢弃 %d 条: %v", len(msgBatch), err)
		}
		msgBatch = msgBatch[:0]

		// 批次落库成功后，提交本批最后一条的位移
		if needCommit {
			if err := w.reader.CommitMessages(ctx, lastMsg); err != nil {
				log.Printf("[Archiver] 位移提交失败: %v", err)
			}
			needCommit = false
		}
	}

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			flush()
			log.Println("[Archiver] 消费者安全退出")
			return
		default:
		}

		// 带超时的单条拉取，兼顾攒批响应
		fetchCtx, cancel := context.WithTimeout(ctx, flushInterval)
		m, err := w.reader.FetchMessage(fetchCtx)
		cancel()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				// 超时或取消 → 刷盘
				flush()
				if errors.Is(err, context.Canceled) {
					return
				}
				continue
			}
			log.Printf("[Archiver] Kafka 拉取异常: %v", err)
			continue
		}

		var raw kafkaMessage
		if err := json.Unmarshal(m.Value, &raw); err != nil {
			_ = w.reader.CommitMessages(ctx, m)
			continue
		}
		roomID, err := strconv.ParseInt(raw.RoomID, 10, 64)
		if err != nil {
			_ = w.reader.CommitMessages(ctx, m)
			continue
		}

		msgBatch = append(msgBatch, &models.Message{
			ID:          nextID(),
			RoomID:      roomID,
			SenderID:    raw.SenderID,
			ClientMsgID: raw.ClientMsgID,
			Type:        1,
			Content:     raw.Content,
			CreatedAt:   time.Now(),
		})
		lastMsg = m
		needCommit = true

		if len(msgBatch) >= batchSize {
			flush()
		}
	}
}
