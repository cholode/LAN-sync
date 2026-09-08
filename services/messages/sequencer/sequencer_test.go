package sequencer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	protocol "lan-im-go/contracts/events"
)

func input(client string) protocol.MessageEnvelope {
	return protocol.MessageEnvelope{RoomID: 1, SenderID: 2, ClientMsgID: client, Content: "你好", Type: 1, CreatedAt: time.Now()}
}

func TestAssignRetryAndRoomIsolation(t *testing.T) {
	s := New()
	a, err := s.Assign(input("C1"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Assign(input("C1"))
	if err != nil {
		t.Fatal(err)
	}
	if a != b || a.RoomSeq != 1 || a.MessageID == 0 {
		t.Fatal("重试改变了正式消息")
	}
	c, _ := s.Assign(input("C2"))
	other := input("C3")
	other.RoomID = 9
	d, _ := s.Assign(other)
	if c.RoomSeq != 2 || d.RoomSeq != 1 || c.MessageID <= a.MessageID {
		t.Fatal("序号未按群递增")
	}
	other = input("C1")
	other.SenderID = 3
	e, err := s.Assign(other)
	if err != nil || e.RoomSeq != 3 {
		t.Fatal("不同用户的客户端凭证被误去重")
	}
	conflict := input("C1")
	conflict.Content = "不同内容"
	if _, err := s.Assign(conflict); err == nil {
		t.Fatal("未拒绝凭证冲突")
	}
	f, _ := s.Assign(input("C4"))
	if f.RoomSeq != 4 {
		t.Fatal("冲突消耗了群序号")
	}
}

func TestConcurrentDuplicateAssign(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	results := make(chan protocol.MessageEnvelope, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m, err := s.Assign(input("same"))
			if err != nil {
				t.Error(err)
				return
			}
			results <- m
		}()
	}
	wg.Wait()
	close(results)
	var first protocol.MessageEnvelope
	for m := range results {
		if first.MessageID == 0 {
			first = m
		}
		if m != first {
			t.Fatal("并发重复请求产生多个编号")
		}
	}
}

type fakeReader struct {
	records        []kafka.Message
	commits        int
	commitFailures int
}

var exhausted = errors.New("测试输入结束")

func (r *fakeReader) FetchMessage(context.Context) (kafka.Message, error) {
	if len(r.records) == 0 {
		return kafka.Message{}, exhausted
	}
	m := r.records[0]
	r.records = r.records[1:]
	return m, nil
}
func (r *fakeReader) CommitMessages(context.Context, ...kafka.Message) error {
	if r.commitFailures > 0 {
		r.commitFailures--
		return errors.New("确认丢失")
	}
	r.commits++
	return nil
}

func TestOutputRetryKeepsIDsAndCommitFollowsOutput(t *testing.T) {
	value, _ := protocol.Marshal(input("C1"))
	r := &fakeReader{records: []kafka.Message{{Value: value, Offset: 1}, {Value: value, Offset: 2}}, commitFailures: 1}
	var outputs []protocol.MessageEnvelope
	err := Run(context.Background(), r, func(_ context.Context, record kafka.Message) error {
		m, err := protocol.Unmarshal(record.Value)
		if err != nil {
			return err
		}
		outputs = append(outputs, m)
		if len(outputs) == 1 {
			if r.commits != 0 {
				t.Fatal("输出前提交了进度")
			}
			return errors.New("写入成功但确认丢失")
		}
		return nil
	}, New())
	if !errors.Is(err, exhausted) || r.commits != 2 || len(outputs) != 3 {
		t.Fatalf("重试过程错误: %v %+v %d", err, r, len(outputs))
	}
	for _, m := range outputs {
		if m != outputs[0] {
			t.Fatal("重投改变了消息编号")
		}
	}
}

func TestRelayRetriesWithoutWaitingForArchive(t *testing.T) {
	m, _ := New().Assign(input("C1"))
	value, _ := protocol.Marshal(m)
	r := &fakeReader{records: []kafka.Message{{Value: value}}}
	attempts := 0
	err := Relay(context.Background(), r, func(_ context.Context, got protocol.MessageEnvelope, _ []byte) error {
		attempts++
		if got.MessageID != m.MessageID || got.RoomSeq != m.RoomSeq || r.commits != 0 {
			t.Fatal("广播改变编号或提前提交")
		}
		if attempts == 1 {
			return errors.New("Redis 暂不可用")
		}
		return nil
	})
	if !errors.Is(err, exhausted) || attempts != 2 || r.commits != 1 {
		t.Fatal("广播重试失败")
	}
}
