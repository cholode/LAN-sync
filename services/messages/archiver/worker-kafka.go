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
	"lan-im-go/shared/observability/metrics"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"
)

type Worker struct {
	reader    messageReader
	rdb       *redis.Client
	topic     string
	partition int
	offsetKey string
	saveBatch func([]*models.Message) error
}

// messageReader 隔离 Kafka 客户端，便于验证停机刷新与批次提交顺序。
type messageReader interface {
	ReadMessage(context.Context) (kafka.Message, error)
	Close() error
	Lag() int64
}

func NewWorker(brokers []string, topic string, groupID string, rdb *redis.Client, repo repository.MessageRepository) (*Worker, error) {
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
	// 直读固定分区时显式设置绝对游标，避免重启后 Reader 忽略起始配置而从头扫描。
	if err := reader.SetOffset(startOffset); err != nil {
		_ = reader.Close()
		return nil, fmt.Errorf("设置 Kafka 起始游标失败: %w", err)
	}

	return &Worker{
		reader:    reader,
		rdb:       rdb,
		topic:     topic,
		partition: partition,
		offsetKey: offsetKey,
		saveBatch: repo.SaveMessageBatch,
	}, nil
}

const (
	batchSize         = 1000
	flushInterval     = 500 * time.Millisecond
	roomLatestKeyPref = "im:room:latest:v2:"
	roomLatestTTL     = 30 * time.Minute
	roomLatestMax     = 100
)

type cachedMsg struct {
	RoomSeq     int64     `json:"room_seq,string"`
	ClientMsgID string    `json:"client_msg_id"`
	ID          int64     `json:"id,string"`
	RoomID      int64     `json:"room_id,string"`
	SenderID    int64     `json:"sender_id,string"`
	Type        int8      `json:"type"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
}

func (w *Worker) pushLatestToRedis(ctx context.Context, msgs []*models.Message) {
	if w.rdb == nil || len(msgs) == 0 {
		return
	}
	pipe := w.rdb.Pipeline()
	rooms := make(map[int64]struct{}, len(msgs))

	for _, m := range msgs {
		if m.DeletedAt != 0 || m.RoomSeq <= 0 || m.RoomSeq > 1<<53 {
			continue
		}
		payload, err := json.Marshal(cachedMsg{
			RoomSeq: m.RoomSeq, ClientMsgID: m.ClientMsgID,
			ID: m.ID, RoomID: m.RoomID, SenderID: m.SenderID,
			Type: m.Type, Content: m.Content, CreatedAt: m.CreatedAt,
		})
		if err != nil {
			continue
		}
		key := fmt.Sprintf("%s%d", roomLatestKeyPref, m.RoomID)
		// 相同正式消息覆盖同一成员，旧消息重放不会改变群序号排序。
		pipe.ZAdd(ctx, key, &redis.Z{Score: float64(m.RoomSeq), Member: string(payload)})
		pipe.ZRemRangeByRank(ctx, key, 0, -roomLatestMax-1)
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

func (w *Worker) saveOffset(ctx context.Context, offset int64) error {
	if w.rdb == nil {
		return nil
	}
	return w.rdb.Set(ctx, w.offsetKey, offset, 0).Err()
}

func (w *Worker) Start(ctx context.Context) error {
	defer w.reader.Close()

	pkg.Infof("[Archiver] 稳态消费者已启动（分区直读模式 1000条/500ms + Redis offset）")

	msgBatch := make([]*models.Message, 0, batchSize)
	var lastOffset int64

	flush := func(flushCtx context.Context) error {
		if len(msgBatch) == 0 {
			return nil
		}
		// 已开始的批次使用独立短超时，停机不能在落库成功后取消游标提交。
		flushCtx, stop := context.WithTimeout(context.WithoutCancel(flushCtx), 20*time.Second)
		defer stop()
		count := len(msgBatch)
		// 同一分区串行落库和提交游标，禁止后续批次越过尚未落库的消息。
		if err := w.saveBatch(msgBatch); err != nil {
			return fmt.Errorf("归档批次写入失败: %w", err)
		}
		if err := search.IndexMessages(flushCtx, msgBatch); err != nil {
			pkg.Warnf("[Archiver] 搜索索引写入失败，主存储已完成: %v", err)
		}
		w.pushLatestToRedis(flushCtx, msgBatch)
		if err := w.saveOffset(flushCtx, lastOffset); err != nil {
			return fmt.Errorf("归档游标保存失败: %w", err)
		}
		pkg.Infof("[Archiver] 批量写入成功: %d 条 offset=%d", count, lastOffset)
		msgBatch = msgBatch[:0]
		return nil
	}

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// 使用独立退出上下文，使已接收消息在取消消费后仍能完成刷新。
			flushCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if err := flush(flushCtx); err != nil {
				return err
			}
			pkg.Infoln("[Archiver] 消费者安全退出")
			return nil
		case <-ticker.C:
			if err := flush(ctx); err != nil {
				return err
			}
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
			return fmt.Errorf("正式消息解析失败 offset=%d: %w", m.Offset, err)
		}

		if envelope.MessageID <= 0 || envelope.RoomSeq <= 0 {
			return fmt.Errorf("正式消息缺少编号 offset=%d，禁止归档时重新编号", m.Offset)
		}
		msgBatch = append(msgBatch, &models.Message{
			ID:          envelope.MessageID,
			RoomSeq:     envelope.RoomSeq,
			RoomID:      envelope.RoomID,
			SenderID:    envelope.SenderID,
			ClientMsgID: envelope.ClientMsgID,
			Type:        envelope.Type,
			Content:     envelope.Content,
			CreatedAt:   envelope.CreatedAt,
		})
		lastOffset = m.Offset

		if len(msgBatch) >= batchSize {
			if err := flush(ctx); err != nil {
				return err
			}
		}
	}
}
