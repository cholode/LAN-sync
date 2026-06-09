package archiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
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
	Timestamp   int64  `json:"timestamp"`
}

var idSeq int64

func nextID() int64 {
	now := time.Now().UnixMilli()
	seq := atomic.AddInt64(&idSeq, 1) % 1000
	return now*1000 + seq
}

type Worker struct {
	reader *kafka.Reader
	rdb    *redis.Client // 注入 Redis 客户端，用于热点消息缓存
}

func NewWorker(brokers []string, topic string, groupID string, rdb *redis.Client) *Worker {
	return &Worker{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			GroupID:  groupID,
			Topic:    topic,
			MinBytes: 10e3,
			MaxBytes: 10e6,
			MaxWait:  500 * time.Millisecond,
		}),
		rdb: rdb,
	}
}

const (
	batchSize         = 1000
	flushInterval     = 500 * time.Millisecond
	roomLatestKeyPref = "im:room:latest:"
	roomLatestTTL     = 30 * time.Minute
	roomLatestMax     = 100
)

type cachedMsg struct {
	ID        int64     `json:"id,string"`
	RoomID    int64     `json:"room_id,string"`
	SenderID  int64     `json:"sender_id,string"`
	Type      int8      `json:"type"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func (w *Worker) pushLatestToRedis(ctx context.Context, msgs []*models.Message) {
	if w.rdb == nil || len(msgs) == 0 {
		return
	}
	pipe := w.rdb.Pipeline()
	rooms := make(map[int64]struct{}, len(msgs))

	for _, m := range msgs {
		payload, err := json.Marshal(cachedMsg{
			ID: m.ID, RoomID: m.RoomID, SenderID: m.SenderID,
			Type: m.Type, Content: m.Content, CreatedAt: m.CreatedAt,
		})
		if err != nil {
			continue
		}
		key := fmt.Sprintf("%s%d", roomLatestKeyPref, m.RoomID)
		pipe.LPush(ctx, key, string(payload))
		pipe.LTrim(ctx, key, 0, roomLatestMax-1)
		rooms[m.RoomID] = struct{}{}
	}

	for roomID := range rooms {
		key := fmt.Sprintf("%s%d", roomLatestKeyPref, roomID)
		pipe.Expire(ctx, key, roomLatestTTL)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("[Archiver] Redis 热点缓存写入失败: %v", err)
	}
}

func (w *Worker) Start(ctx context.Context) {
	defer w.reader.Close()

	log.Println("[Archiver] 稳态消费者已启动（微批模式: 1000条/500ms + Redis热点缓存）")

	msgBatch := make([]*models.Message, 0, batchSize)
	var lastMsg kafka.Message
	needCommit := false

	flush := func() {
		if len(msgBatch) == 0 {
			return
		}
		count := len(msgBatch)
		err := repository.Message.SaveMessageBatch(msgBatch)

		if err != nil {
			msgBatch = msgBatch[:0]
			log.Printf("[Archiver] 批量写入失败，%d 条不提交位移，重启后重放: %v", count, err)
			needCommit = false
			return
		}

		// MySQL 成功 → 推入 Redis 热点缓存
		w.pushLatestToRedis(ctx, msgBatch)

		msgBatch = msgBatch[:0]

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

		fetchCtx, cancel := context.WithTimeout(ctx, flushInterval)
		m, err := w.reader.FetchMessage(fetchCtx)
		cancel()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
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
			CreatedAt:   parseTime(raw.Timestamp),
		})
		lastMsg = m
		needCommit = true

		if len(msgBatch) >= batchSize {
			flush()
		}
	}
}

func parseTime(ns int64) time.Time {
	if ns == 0 {
		return time.Now()
	}
	return time.Unix(0, ns)
}
