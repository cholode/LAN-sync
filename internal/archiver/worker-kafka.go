package archiver

import (
	"context"
	"errors"
	"log"

	"github.com/segmentio/kafka-go"
)

type MessageRepository interface {
	Save(ctx context.Context, payload []byte) error
}

type Worker struct {
	reader *kafka.Reader
	repo   MessageRepository
}

func NewWorker(brokers []string, topic string, groupID string, repo MessageRepository) *Worker {
	return &Worker{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			GroupID:  groupID,
			Topic:    topic,
			MinBytes: 10e3,
			MaxBytes: 10e6,
		}),
		repo: repo,
	}
}

func (w *Worker) Start(ctx context.Context) {
	defer w.reader.Close()

	log.Println("消费者已启动...")

	for {

		//阻塞拉取：受控于外部传入的 Context
		m, err := w.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("收到系统canceled信号，退出循环:%v\n", err)
				return
			}
			log.Printf("Kafka 物理拉取异常，进入重试：%v\n", err)
			continue
		}

		//执行业务逻辑：持久化到数据库
		err = w.repo.Save(ctx, m.Value)
		if err != nil {
			log.Printf("kafka持久化失败：%v \n", err)
			continue
		}

		//只有当底层数据库极其明确地返回成功后，才向 Kafka 提交物理位移！
		if err := w.reader.CommitMessages(ctx, m); err != nil {
			log.Printf("位移提交阻断:% v \n", err)
		}
	}
}
