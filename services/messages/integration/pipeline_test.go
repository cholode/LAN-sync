package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	protocol "lan-im-go/contracts/events"
	"lan-im-go/models"
	"lan-im-go/repository"
	messages "lan-im-go/services/messages/api"
	"lan-im-go/services/messages/archiver"
	"lan-im-go/services/messages/producer"
	"lan-im-go/services/messages/sequencer"
)

type bothStores struct {
	repository.MessageRepository
	second repository.MessageRepository
}

func (s bothStores) SaveMessageBatch(batch []*models.Message) error {
	if err := s.MessageRepository.SaveMessageBatch(batch); err != nil {
		return err
	}
	return s.second.SaveMessageBatch(batch)
}

// 仅连接显式提供的隔离环境；数据库应为一次性测试数据库。
func TestSequencedPipeline(t *testing.T) {
	if os.Getenv("MESSAGE_PIPELINE_ISOLATED") != "yes" {
		t.Skip("未启用隔离消息链路测试")
	}
	broker, dsn, uri, redisAddr := os.Getenv("MESSAGE_TEST_BROKER"), os.Getenv("MESSAGE_TEST_MYSQL_DSN"), os.Getenv("MESSAGE_TEST_MONGO_URI"), os.Getenv("MESSAGE_TEST_REDIS_ADDR")
	if broker == "" || dsn == "" || uri == "" || redisAddr == "" {
		t.Fatal("隔离测试连接配置不完整")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	topic := fmt.Sprintf("im-sequence-test-%d", time.Now().UnixNano())
	ingress := topic + "-input"
	if err := sequencer.EnsureTopics(ctx, []string{broker}, ingress, topic); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn, err := kafka.Dial("tcp", broker)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		controller, err := conn.Controller()
		if err != nil {
			t.Error(err)
			return
		}
		admin, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
		if err != nil {
			t.Error(err)
			return
		}
		defer admin.Close()
		if err := admin.DeleteTopics(ingress, topic); err != nil {
			t.Error(err)
		}
	})
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := db.AutoMigrate(&models.Message{}); err != nil {
		t.Fatal(err)
	}
	mysqlRepo := messages.NewMySQLRepository(db)
	mc, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer mc.Disconnect(context.Background())
	collection := mc.Database("message_sequence_test").Collection(topic)
	defer collection.Drop(context.Background())
	_, err = collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "sender_id", Value: 1}, {Key: "client_msg_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "room_id", Value: 1}, {Key: "room_seq", Value: 1}}, Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"room_seq": bson.M{"$gt": 0}})},
	})
	if err != nil {
		t.Fatal(err)
	}
	mongoRepo := messages.NewMongoRepository(collection)
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()
	newReader := func(name, group string) *kafka.Reader {
		return kafka.NewReader(kafka.ReaderConfig{Brokers: []string{broker}, Topic: name, GroupID: group, MinBytes: 1, MaxBytes: 10e6, MaxWait: 10 * time.Millisecond})
	}
	reader := newReader(ingress, topic+"-seq")
	defer reader.Close()
	writer := &kafka.Writer{Addr: kafka.TCP(broker), Topic: topic, RequiredAcks: kafka.RequireAll, BatchTimeout: time.Millisecond}
	defer writer.Close()
	seqCtx, seqCancel := context.WithCancel(ctx)
	defer seqCancel()
	seqDone := make(chan error, 1)
	var attempts atomic.Int32
	go func() {
		seqDone <- sequencer.Run(seqCtx, reader, func(ctx context.Context, m kafka.Message) error {
			if err := writer.WriteMessages(ctx, m); err != nil {
				return err
			}
			// 模拟已经写入正式 Kafka，但生产确认丢失，再次输出必须复用编号。
			if attempts.Add(1) == 1 {
				return errors.New("模拟 Kafka 确认丢失")
			}
			return nil
		}, sequencer.New())
	}()
	relayReader := newReader(topic, topic+"-relay")
	defer relayReader.Close()
	pubsub := rdb.Subscribe(ctx, topic)
	defer pubsub.Close()
	if _, err := pubsub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	ch := pubsub.Channel()
	relayCtx, relayCancel := context.WithCancel(ctx)
	defer relayCancel()
	relayDone := make(chan error, 1)
	go func() {
		relayDone <- sequencer.Relay(relayCtx, relayReader, func(ctx context.Context, _ protocol.MessageEnvelope, value []byte) error {
			return rdb.Publish(ctx, topic, value).Err()
		})
	}()
	client := producer.NewMessageClient([]string{broker}, ingress)
	defer client.Close()
	for _, req := range []struct {
		user      int
		key, text string
	}{{2, "C1", "你好"}, {2, "C1", "你好"}, {3, "C1", "我很好"}, {2, "C2", "第三条"}} {
		if err := client.HandleIncomingMessage(ctx, "1", req.user, req.text, req.key); err != nil {
			t.Fatal(err)
		}
	}
	var observed []protocol.MessageEnvelope
	for len(observed) < 5 {
		select {
		case item := <-ch:
			m, err := protocol.Unmarshal([]byte(item.Payload))
			if err != nil {
				t.Fatal(err)
			}
			observed = append(observed, m)
		case <-ctx.Done():
			t.Fatal("编号广播超时", ctx.Err())
		}
	}
	seqCancel()
	relayCancel()
	<-seqDone
	<-relayDone
	if observed[0].MessageID != observed[1].MessageID || observed[0].MessageID != observed[2].MessageID || observed[3].RoomSeq != 2 || observed[4].RoomSeq != 3 {
		t.Fatalf("编号或重投错误: %+v", observed)
	}
	// 广播完成后才启动归档，证明数据库不在实时推送的依赖路径上。
	var before int64
	db.Model(&models.Message{}).Where("room_id = ?", 1).Count(&before)
	if before != 0 {
		t.Fatal("测试数据库并非空白隔离数据库")
	}
	defer db.Unscoped().Where("room_id = ?", 1).Delete(&models.Message{})
	worker, err := archiver.NewWorker([]string{broker}, topic, "unused", rdb, bothStores{mysqlRepo, mongoRepo})
	if err != nil {
		t.Fatal(err)
	}
	archiveCtx, archiveCancel := context.WithCancel(ctx)
	defer archiveCancel()
	archiveDone := make(chan error, 1)
	go func() { archiveDone <- worker.Start(archiveCtx) }()
	for {
		rows, err := mongoRepo.GetHistoryByCursor(1, 0, 20)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 3 {
			break
		}
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			t.Fatal("归档超时")
		}
	}
	archiveCancel()
	if err := <-archiveDone; err != nil {
		t.Fatal(err)
	}
	for name, repo := range map[string]repository.MessageRepository{"mysql": mysqlRepo, "mongo": mongoRepo} {
		t.Run(name, func(t *testing.T) {
			rows, err := repo.GetHistoryByCursor(1, 0, 20)
			if err != nil || len(rows) != 3 {
				t.Fatalf("落库未去重: %v %d", err, len(rows))
			}
			for i, m := range rows {
				if m.RoomSeq != int64(i+1) {
					t.Fatal("群序号错误")
				}
			}
			original := *rows[0]
			if err := repo.SaveMessageBatch([]*models.Message{&original, &original}); err != nil {
				t.Fatal(err)
			}
			conflict := original
			conflict.ID++
			conflict.Content = "冲突内容"
			if err := repo.SaveMessageBatch([]*models.Message{&conflict}); err == nil {
				t.Fatal("正式消息冲突未被拒绝")
			}
			older, err := repo.GetHistoryByCursor(1, rows[2].ID, 1)
			if err != nil || len(older) != 1 || older[0].RoomSeq != 2 {
				t.Fatal("群序号分页错误", err)
			}
		})
	}
	// 历史时间倒置也不能改变群序号排序。
	if err := db.Model(&models.Message{}).Where("room_id = ? AND room_seq = ?", 1, 1).Update("created_at", time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	rows, err := mysqlRepo.GetHistoryByCursor(1, 0, 20)
	if err != nil || rows[0].RoomSeq != 1 {
		t.Fatal("历史错误地按时间排序")
	}
}
