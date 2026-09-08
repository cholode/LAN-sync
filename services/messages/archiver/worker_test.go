package archiver

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	protocol "lan-im-go/contracts/events"
	"lan-im-go/models"
)

type readerStub struct {
	messages []kafka.Message
	reads    int
	closed   bool
	cancel   context.CancelFunc
}

func (r *readerStub) ReadMessage(ctx context.Context) (kafka.Message, error) {
	if r.reads == len(r.messages) {
		r.cancel()
		<-ctx.Done()
		return kafka.Message{}, ctx.Err()
	}
	m := r.messages[r.reads]
	r.reads++
	return m, nil
}
func (r *readerStub) Close() error { r.closed = true; return nil }
func (r *readerStub) Lag() int64   { return 0 }

func records(t *testing.T, count int) []kafka.Message {
	t.Helper()
	out := make([]kafka.Message, count)
	for i := range out {
		value, err := protocol.Marshal(protocol.MessageEnvelope{MessageID: int64(i + 100), RoomSeq: int64(i + 1), RoomID: 1, SenderID: 2,
			ClientMsgID: fmt.Sprintf("test-%d", i), Content: "归档测试", Type: 1, CreatedAt: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		out[i] = kafka.Message{Offset: int64(i), Value: value}
	}
	return out
}

func TestWorkerFlushesPartialBatchBeforeExit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &readerStub{messages: records(t, 3), cancel: cancel}
	var saved []*models.Message
	w := &Worker{reader: r, topic: "test", saveBatch: func(batch []*models.Message) error {
		saved = append(saved, batch...)
		return nil
	}}
	if err := w.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if !r.closed || len(saved) != 3 {
		t.Fatalf("退出前未保存完整批次: closed=%v saved=%d", r.closed, len(saved))
	}
	for i, m := range saved {
		if m.ID != int64(i+100) || m.RoomSeq != int64(i+1) {
			t.Fatal("归档改写了编号器的结果")
		}
		if m.ClientMsgID != fmt.Sprintf("test-%d", i) {
			t.Fatal("消息顺序错误")
		}
	}
}

func TestWorkerStopsBeforeReadingNextBatchOnWriteFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &readerStub{messages: records(t, batchSize+1), cancel: cancel}
	want := errors.New("数据库暂不可用")
	w := &Worker{reader: r, topic: "test", saveBatch: func([]*models.Message) error { return want }}
	if err := w.Start(ctx); !errors.Is(err, want) {
		t.Fatalf("未返回写入错误: %v", err)
	}
	if r.reads != batchSize || !r.closed {
		t.Fatalf("写入失败后仍继续消费: reads=%d closed=%v", r.reads, r.closed)
	}
}
