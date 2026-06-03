package archiver

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/segmentio/kafka-go"
	"lan-im-go/models"
	"lan-im-go/repository"
	"log"
	"sync/atomic"
	"time"
)

// kafkaMessage 是 Kafka 消息的中间表示，JSON key 对齐生产者使用的 snake_case 格式
type kafkaMessage struct {
	RoomID      int64  `json:"room_id"`
	SenderID    int64  `json:"sender_id"`
	Content     string `json:"content"`
	ClientMsgID string `json:"client_msg_id"`
}

var idSeq int64 // 毫秒内的序列号，配合时间戳生成唯一 ID

func nextID() int64 {
	now := time.Now().UnixMilli()
	seq := atomic.AddInt64(&idSeq, 1) % 1000
	return now*1000 + seq
}

type MessageRepository interface {
	Save(ctx context.Context, payload []byte) error
}

type Worker struct {
	reader *kafka.Reader
	repo   MessageRepository
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

func (w *Worker) Start(ctx context.Context) {
	defer w.reader.Close()

	log.Println("稳态消费者已启动...")

	for {
		m, err := w.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("收到系统 canceled 信号，退出循环 %v\n", err)
				return
			}
			log.Printf("Kafka 物理拉取异常，进入重试 %v\n", err)
			continue
		}

		// 1. 先反序列化到中间结构体（snake_case JSON → Go struct）
		var raw kafkaMessage
		if err := json.Unmarshal(m.Value, &raw); err != nil {
			log.Printf("Kafka 脏数据解析阻断，直接抛弃并跳过 %v\n", err)
			if commitErr := w.reader.CommitMessages(ctx, m); commitErr != nil {
				log.Printf("毒消息位移跳过失败 %v\n", commitErr)
			}
			continue
		}

		// 2. 转换为业务模型 + 生成 ID
		msg := &models.Message{
			ID:          nextID(),
			RoomID:      raw.RoomID,
			SenderID:    raw.SenderID,
			ClientMsgID: raw.ClientMsgID,
			Type:        1,
			Content:     raw.Content,
			CreatedAt:   time.Now(),
		}

		// 3. 持久化
		err = repository.Message.SaveMessage(msg)
		if err != nil {
			log.Printf("底层数据库持久化物理阻断，拒绝提交位移: %v \n", err)
			continue
		}

		// 4. 提交位移
		if err := w.reader.CommitMessages(ctx, m); err != nil {
			log.Printf("位移提交物理阻断: %v \n", err)
		}
	}
}