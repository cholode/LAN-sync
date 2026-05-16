package archiver

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/segmentio/kafka-go"
	"lan-im-go/models"
	"lan-im-go/repository"
	"log"
	"time"
)

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
		// 1. 阻塞拉取：受控于外部传入的 Context
		m, err := w.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("收到系统 canceled 信号，退出循环: %v\n", err)
				return
			}
			log.Printf("Kafka 物理拉取异常，进入重试: %v\n", err)
			continue
		}

		// 2. 数据的物化与反序列化
		var msg models.Message
		if err := json.Unmarshal(m.Value, &msg); err != nil {
			// 如果由于前端发送了非法结构，导致这里解析失败，绝对不能 return，也不能单纯 continue。
			// 必须立刻提交这段脏数据的位移！否则消费者会陷入无限拉取这条坏消息的死循环，导致整个分区瘫痪。
			log.Printf("Kafka 脏数据解析阻断，直接抛弃并跳过: %v\n", err)
			if commitErr := w.reader.CommitMessages(ctx, m); commitErr != nil {
				log.Printf("毒消息位移跳过失败: %v\n", commitErr)
			}
			continue
		}

		// 3. 调用全局单例持久化到数据库
		err = repository.Message.SaveMessage(&msg)
		if err != nil {
			// 数据库宕机、死锁或写入超时
			// 此时绝对不能提交位移，必须保留 Kafka 游标，依靠下一次循环重试保障数据绝对不丢失
			log.Printf("底层数据库持久化物理阻断，拒绝提交位移: %v \n", err)
			continue
		}

		// 4. 只有当底层数据库极其明确地返回成功后，才向 Kafka 提交物理位移
		if err := w.reader.CommitMessages(ctx, m); err != nil {
			log.Printf("位移提交物理阻断: %v \n", err)
		}
	}
}
