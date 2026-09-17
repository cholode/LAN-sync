package sequencer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	protocol "lan-im-go/contracts/events"
)

type batchReader struct {
	records         []kafka.Message
	commits         [][]kafka.Message
	waitForDeadline bool
}

func (r *batchReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if len(r.records) > 0 {
		first := r.records[0]
		r.records = r.records[1:]
		return first, nil
	}
	if r.waitForDeadline && len(r.commits) == 0 {
		<-ctx.Done()
		return kafka.Message{}, ctx.Err()
	}
	return kafka.Message{}, io.EOF
}
func (r *batchReader) CommitMessages(_ context.Context, records ...kafka.Message) error {
	r.commits = append(r.commits, append([]kafka.Message(nil), records...))
	return nil
}
func batchRecords(n int) []kafka.Message {
	result := make([]kafka.Message, n)
	for i := range result {
		msg := throughputInput(i)
		msg.RoomID = 1
		value, _ := protocol.Marshal(msg)
		result[i] = kafka.Message{Value: value, Offset: int64(i)}
	}
	return result
}
func TestBatchRetriesIdenticalOutputBeforeCommit(t *testing.T) {
	r := &batchReader{records: batchRecords(5)}
	var firstAttempt [][]byte
	attempts := 0
	var sizes []int
	err := RunBatched(context.Background(), r, func(_ context.Context, output ...kafka.Message) error {
		attempts++
		if attempts == 1 {
			for _, m := range output {
				firstAttempt = append(firstAttempt, append([]byte(nil), m.Value...))
			}
			return errors.New("ack lost")
		}
		if attempts == 2 {
			if len(r.commits) != 0 {
				t.Fatal("committed before write acknowledgement")
			}
			for i, m := range output {
				if !bytes.Equal(m.Value, firstAttempt[i]) {
					t.Fatal("retry changed numbering")
				}
			}
		}
		for i, m := range output {
			decoded, _ := protocol.Unmarshal(m.Value)
			if decoded.RoomSeq != int64(len(sizes)*2+i+1) {
				t.Fatal("order changed")
			}
		}
		sizes = append(sizes, len(output))
		return nil
	}, New(), BatchOptions{MaxMessages: 2, MaxBytes: 1024, MaxWait: time.Millisecond})
	if !errors.Is(err, io.EOF) || len(sizes) != 3 || sizes[2] != 1 || len(r.commits) != 3 {
		t.Fatalf("tail batch lost: sizes=%v commits=%d err=%v", sizes, len(r.commits), err)
	}
	for i, batch := range r.commits {
		if batch[len(batch)-1].Offset != int64(min((i+1)*2, 5)-1) {
			t.Fatal("wrong committed offset")
		}
	}
}
func TestBatchFlushesOnDeadline(t *testing.T) {
	r := &batchReader{records: batchRecords(1), waitForDeadline: true}
	start := time.Now()
	err := RunBatched(context.Background(), r, func(_ context.Context, records ...kafka.Message) error {
		if len(records) != 1 {
			t.Fatal("wrong batch")
		}
		return nil
	}, New(), BatchOptions{MaxMessages: 100, MaxBytes: 1024, MaxWait: 10 * time.Millisecond})
	if !errors.Is(err, io.EOF) || len(r.commits) != 1 || time.Since(start) < 10*time.Millisecond {
		t.Fatalf("deadline flush failed: %v", err)
	}
}
func TestBatchByteLimitPreservesPendingRecord(t *testing.T) {
	records := batchRecords(3)
	limit := len(records[0].Value) + 1
	r := &batchReader{records: records}
	writes := 0
	err := RunBatched(context.Background(), r, func(_ context.Context, batch ...kafka.Message) error {
		writes++
		if len(batch) != 1 {
			t.Fatal("byte cap exceeded")
		}
		return nil
	}, New(), BatchOptions{MaxMessages: 100, MaxBytes: limit, MaxWait: time.Millisecond})
	if !errors.Is(err, io.EOF) || writes != 3 || len(r.commits) != 3 {
		t.Fatalf("pending message lost: %v", err)
	}
}
func TestBatchFailedOutputDoesNotCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &batchReader{records: batchRecords(2)}
	err := RunBatched(ctx, r, func(context.Context, ...kafka.Message) error { cancel(); return errors.New("write failed") }, New(), BatchOptions{MaxMessages: 2, MaxBytes: 1024, MaxWait: time.Millisecond})
	if !errors.Is(err, context.Canceled) || len(r.commits) != 0 {
		t.Fatalf("failed output committed: %v", err)
	}
}
