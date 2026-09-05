package archiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"lan-im-go/contracts/events"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"lan-im-go/repository"
	"lan-im-go/services/messages/search"
	"lan-im-go/shared/concurrency/taskpool"
	"lan-im-go/shared/observability/metrics"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"
)

// idSeq 使用雪花算法简化版：毫秒时间戳(42位) + 序列号(10位)
// 支持每毫秒 1024 个 ID，可用约 140 年
var (
	idEpoch    int64 = 1750000000000 // 2025-06-01 00:00:00 UTC 起始时间戳(毫秒)
	idSequence int64
	idLastMs   int64
	idMu       sync.Mutex
)

func nextID() int64 {
	idMu.Lock()
	defer idMu.Unlock()

	now := time.Now().UnixMilli()
	if now == idLastMs {
		idSequence = (idSequence + 1) & 0x3FF // 10 位，可用范围为 0–1023
		if idSequence == 0 {
			for now <= idLastMs {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		idSequence = 0
	}
	idLastMs = now

	return (now-idEpoch)<<10 | idSequence
}

type Worker struct {
	reader    *kafka.Reader
	rdb       *redis.Client
	topic     string
	partition int
	offsetKey string
}

func NewWorker(brokers []string, topic string, groupID string, rdb *redis.Client) *Worker {
	const partition = 0
	offsetKey := fmt.Sprintf("im:kafka:offset:{%s}:%d", topic, partition)

	startOffset := int64(kafka.FirstOffset)
	if rdb != nil {
		val, err := rdb.Get(context.Background(), offsetKey).Result()
		if err == nil {
			if saved, parseErr := strconv.ParseInt(val, 10, 64); parseErr == nil {
				startOffset = saved + 1
				pkg.Infof("[Archiver] 从 Redis 恢复 offset=%d, 起始位置=%d", saved, startOffset)
			}
		}
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       topic,
		Partition:   partition,
		MinBytes:    10e3,
		MaxBytes:    10e6,
		MaxWait:     500 * time.Millisecond,
		StartOffset: startOffset,
	})

	return &Worker{
		reader:    reader,
		rdb:       rdb,
		topic:     topic,
		partition: partition,
		offsetKey: offsetKey,
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
		pkg.Infof("[Archiver] Redis 热点缓存写入失败: %v", err)
	}
}

func (w *Worker) saveOffset(ctx context.Context, offset int64) {
	if w.rdb == nil {
		return
	}
	if err := w.rdb.Set(ctx, w.offsetKey, offset, 0).Err(); err != nil {
		pkg.Infof("[Archiver] Redis offset 保存失败: %v", err)
	}
}

func (w *Worker) Start(ctx context.Context) {
	defer w.reader.Close()

	pkg.Infof("[Archiver] 稳态消费者已启动（分区直读模式 1000条/500ms + Redis offset）")

	msgBatch := make([]*models.Message, 0, batchSize)
	var lastOffset int64

	flush := func() {
		if len(msgBatch) == 0 {
			return
		}
		count := len(msgBatch)
		// 拷贝批次数据，立即清空原切片让消费者继续读取
		batch := make([]*models.Message, count)
		copy(batch, msgBatch)
		savedOffset := lastOffset
		msgBatch = msgBatch[:0]

		// 通过协程池异步写入 MySQL，避免阻塞 Kafka 消费循环
		taskpool.Go(func() {
			err := repository.Message.SaveMessageBatch(batch)
			if err != nil {
				pkg.Infof("[Archiver] 批量写入失败，%d 条 %v", count, err)
				return
			}

			if indexErr := search.IndexMessages(ctx, batch); indexErr != nil {
				pkg.Warnf("[Archiver] Elasticsearch index failed: %d msgs: %v", count, indexErr)
			} else {
				pkg.Infof("[Archiver] Elasticsearch indexed: %d msgs offset=%d", count, savedOffset)
			}

			w.pushLatestToRedis(ctx, batch)
			pkg.Infof("[Archiver] 批量写入成功: %d 条 offset=%d", count, savedOffset)

			w.saveOffset(ctx, savedOffset)
		})
	}

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			flush()
			pkg.Infoln("[Archiver] 消费者安全退出")
			return
		case <-ticker.C:
			flush()
		default:
		}

		readStart := time.Now()
		readCtx, cancel := context.WithTimeout(ctx, flushInterval)
		m, err := w.reader.ReadMessage(readCtx)
		cancel()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			metrics.ObserveKafkaReadError(w.topic, err)
			pkg.Infof("[Archiver] Kafka 读取异常: %v", err)
			continue
		}
		metrics.ObserveKafkaConsume(w.topic, readStart, nil)
		metrics.SetKafkaConsumerLag(w.topic, w.partition, w.reader.Lag())

		envelope, err := protocol.Unmarshal(m.Value)
		if err != nil {
			pkg.Infof("[Archiver] message parse failed offset=%d: %v", m.Offset, err)
			continue
		}

		msgBatch = append(msgBatch, &models.Message{
			ID:          nextID(),
			RoomID:      envelope.RoomID,
			SenderID:    envelope.SenderID,
			ClientMsgID: envelope.ClientMsgID,
			Type:        envelope.Type,
			Content:     envelope.Content,
			CreatedAt:   envelope.CreatedAt,
		})
		lastOffset = m.Offset

		if len(msgBatch) >= batchSize {
			flush()
		}
	}
}
