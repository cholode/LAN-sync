package sequencer

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	protocol "lan-im-go/contracts/events"
)

func relayRecords(n int) []kafka.Message {
	records := batchRecords(n)
	state := New()
	for i := range records {
		input, _ := protocol.Unmarshal(records[i].Value)
		msg, _ := state.Assign(input)
		records[i].Value, _ = protocol.Marshal(msg)
	}
	return records
}

type relayCommitRetryReader struct {
	*batchReader
	attempts int
}

func (r *relayCommitRetryReader) CommitMessages(ctx context.Context, records ...kafka.Message) error {
	r.attempts++
	if r.attempts == 1 {
		return errors.New("commit failed")
	}
	return r.batchReader.CommitMessages(ctx, records...)
}

func TestRelayBatchCommitRetryDoesNotRepublish(t *testing.T) {
	r := &relayCommitRetryReader{batchReader: &batchReader{records: relayRecords(2)}}
	publishes := 0
	err := RelayBatched(context.Background(), r, func(context.Context, []RelayMessage) error { publishes++; return nil }, BatchOptions{MaxMessages: 2, MaxBytes: 1024, MaxWait: time.Second})
	if !errors.Is(err, io.EOF) || publishes != 1 || r.attempts != 2 || len(r.commits) != 1 {
		t.Fatalf("publishes=%d commitAttempts=%d err=%v", publishes, r.attempts, err)
	}
}

func TestRelayBatchRetryAndTail(t *testing.T) {
	r := &batchReader{records: relayRecords(5)}
	var attempts, successful int
	var first []byte
	err := RelayBatched(context.Background(), r, func(_ context.Context, messages []RelayMessage) error {
		attempts++
		if attempts == 1 {
			first = append([]byte(nil), messages[0].Value...)
			return errors.New("uncertain pipeline acknowledgement")
		}
		if attempts == 2 && (len(r.commits) != 0 || string(first) != string(messages[0].Value)) {
			t.Fatal("retry changed message or committed before publication")
		}
		for _, msg := range messages {
			successful++
			if msg.Message.RoomSeq != int64(successful) {
				t.Fatal("order changed")
			}
		}
		return nil
	}, BatchOptions{MaxMessages: 2, MaxBytes: 1024, MaxWait: time.Second})
	if !errors.Is(err, io.EOF) || attempts != 4 || successful != 5 || len(r.commits) != 3 || len(r.commits[2]) != 1 {
		t.Fatalf("attempts=%d successful=%d commits=%v err=%v", attempts, successful, r.commits, err)
	}
}

func TestRelayBatchBounds(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "bytes", true: "deadline"}[deadline], func(t *testing.T) {
			records := relayRecords(3)
			limit := len(records[0].Value) + 1
			if deadline {
				records = records[:1]
				limit = 1024
			}
			r := &batchReader{records: records, waitForDeadline: deadline}
			count := 0
			err := RelayBatched(context.Background(), r, func(_ context.Context, messages []RelayMessage) error {
				if len(messages) != 1 {
					t.Fatal("batch exceeded byte cap or wrong timeout batch")
				}
				count++
				return nil
			}, BatchOptions{MaxMessages: 100, MaxBytes: limit, MaxWait: 10 * time.Millisecond})
			if !errors.Is(err, io.EOF) || count != len(records) || len(r.commits) != len(records) {
				t.Fatalf("lost records: count=%d err=%v", count, err)
			}
		})
	}
}

func TestRelayBatchFailureDoesNotCommit(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "publish", true: "invalid"}[invalid], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			records := relayRecords(2)
			if invalid {
				records[1] = batchRecords(1)[0]
			}
			r := &batchReader{records: records}
			calls := 0
			err := RelayBatched(ctx, r, func(context.Context, []RelayMessage) error { calls++; cancel(); return errors.New("publish failed") }, BatchOptions{MaxMessages: 2, MaxBytes: 1024, MaxWait: time.Second})
			if err == nil || len(r.commits) != 0 || (invalid && calls != 0) {
				t.Fatalf("err=%v commits=%v calls=%d", err, r.commits, calls)
			}
		})
	}
}
