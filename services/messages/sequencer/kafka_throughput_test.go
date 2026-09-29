package sequencer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	protocol "lan-im-go/contracts/events"
)

// Opt-in integration performance test: creates unique Kafka topics, preloads input,
// then runs the production Run function with the production Kafka client settings.
// It never subscribes to the application's topics, Redis, or database.
func TestKafkaSequencerThroughput(t *testing.T) { runKafkaSequencerThroughput(t, 0) }

func runKafkaSequencerThroughput(t *testing.T, batchSize int) {
	brokerEnv := os.Getenv("KAFKA_SEQUENCER_TEST_BROKERS")
	if brokerEnv == "" {
		t.Skip("set KAFKA_SEQUENCER_TEST_BROKERS to explicitly enable the Kafka throughput test")
	}
	n := 10000
	if raw := os.Getenv("SEQUENCER_PERF_MESSAGES"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 100 || parsed > 100000 {
			t.Fatal("SEQUENCER_PERF_MESSAGES must be between 100 and 100000")
		}
		n = parsed
	}
	brokers := strings.Split(brokerEnv, ",")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	prefix := fmt.Sprintf("seq_perf_%d", time.Now().UnixNano())
	ingress, output := prefix+"_in", prefix+"_out"
	if err := EnsureTopics(ctx, brokers, ingress, output); err != nil {
		t.Fatal(err)
	}
	t.Logf("topics ingress=%s output=%s messages=%d rooms=100 batch_cap=%d", ingress, output, n, batchSize)

	seed := &kafka.Writer{Addr: kafka.TCP(brokers...), Topic: ingress, Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll, BatchSize: 1000, BatchTimeout: time.Millisecond}
	t.Cleanup(func() { _ = seed.Close() })
	seedStart := time.Now()
	for start := 0; start < n; start += 1000 {
		batch := make([]kafka.Message, 0, min(1000, n-start))
		for i := start; i < min(start+1000, n); i++ {
			msg := throughputInput(i)
			value, err := protocol.Marshal(msg)
			if err != nil {
				t.Fatal(err)
			}
			batch = append(batch, kafka.Message{Key: []byte(strconv.FormatInt(msg.RoomID, 10)), Value: value})
		}
		if err := writeSeedBatch(ctx, seed, batch); err != nil {
			t.Fatal(err)
		}
	}
	seedDuration := time.Since(seedStart)
	t.Logf("preload messages=%d seconds=%.3f rate=%.1f/s", n, seedDuration.Seconds(), float64(n)/seedDuration.Seconds())

	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: brokers, Topic: ingress, GroupID: prefix, MinBytes: 1, MaxBytes: 10e6, MaxWait: 20 * time.Millisecond, CommitInterval: 0, StartOffset: kafka.FirstOffset})
	writer := &kafka.Writer{Addr: kafka.TCP(brokers...), Topic: output, Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll, BatchTimeout: 5 * time.Millisecond}
	if batchSize > 0 {
		writer.BatchSize = batchSize
		writer.BatchBytes = 1 << 20
	}
	t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	timed := &throughputReader{Reader: reader, limit: n}
	var writeTime time.Duration
	var peakHeap atomic.Uint64
	runtime.GC()
	var memoryBefore runtime.MemStats
	runtime.ReadMemStats(&memoryBefore)
	peakHeap.Store(memoryBefore.HeapAlloc)
	done := make(chan struct{})
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		ticker := time.NewTicker(50 * time.Millisecond)
		ticks := 0
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				var memory runtime.MemStats
				runtime.ReadMemStats(&memory)
				if memory.HeapAlloc > peakHeap.Load() {
					peakHeap.Store(memory.HeapAlloc)
				}
				ticks++
				if ticks%100 == 0 {
					t.Logf("progress committed=%d/%d heap_MiB=%.2f", timed.completed.Load(), n, float64(memory.HeapAlloc)/(1<<20))
				}
			}
		}
	}()
	var writeCalls, largestBatch, outputCount int
	writeBatch := func(ctx context.Context, messages ...kafka.Message) error {
		began := time.Now()
		err := writer.WriteMessages(ctx, messages...)
		writeTime += time.Since(began)
		writeCalls++
		largestBatch = max(largestBatch, len(messages))
		outputCount += len(messages)
		return err
	}
	start := time.Now()
	var err error
	if batchSize == 0 {
		err = Run(ctx, timed, func(ctx context.Context, message kafka.Message) error { return writeBatch(ctx, message) }, New())
	} else {
		err = RunBatched(ctx, timed, writeBatch, New(), BatchOptions{MaxMessages: batchSize, MaxBytes: 1 << 20, MaxWait: 5 * time.Millisecond})
	}
	elapsed := time.Since(start)
	close(done)
	<-monitorDone
	if !errors.Is(err, io.EOF) || timed.completed.Load() != int64(n) {
		t.Fatalf("pipeline failed: committed=%d err=%v", timed.completed.Load(), err)
	}
	steady := start.Add(elapsed).Sub(timed.firstFetched)
	var memoryAfter runtime.MemStats
	runtime.ReadMemStats(&memoryAfter)
	if memoryAfter.HeapAlloc > peakHeap.Load() {
		peakHeap.Store(memoryAfter.HeapAlloc)
	}
	t.Logf("BATCH cap=%d write_calls=%d largest=%d average=%.2f heap_before_MiB=%.3f alloc_MiB=%.3f", batchSize, writeCalls, largestBatch, float64(outputCount)/float64(writeCalls), float64(memoryBefore.HeapAlloc)/(1<<20), float64(memoryAfter.TotalAlloc-memoryBefore.TotalAlloc)/(1<<20))
	t.Logf("RESULT messages=%d total_seconds=%.6f total_rate=%.2f/s steady_seconds=%.6f steady_rate=%.2f/s startup_fetch_ms=%.3f fetch_ms=%.3f write_ms=%.3f commit_ms=%.3f peak_heap_MiB=%.2f", n, elapsed.Seconds(), float64(n)/elapsed.Seconds(), steady.Seconds(), float64(n)/steady.Seconds(), float64(timed.startup)/float64(time.Millisecond), float64(timed.fetch)/float64(time.Millisecond), float64(writeTime)/float64(time.Millisecond), float64(timed.commit)/float64(time.Millisecond), float64(peakHeap.Load())/(1<<20))

	// Verify the persisted output, including exact count, IDs, per-room order and payload.
	outputReader := kafka.NewReader(kafka.ReaderConfig{Brokers: brokers, Topic: output, Partition: 0, MinBytes: 1, MaxBytes: 10e6, StartOffset: kafka.FirstOffset})
	t.Cleanup(func() { _ = outputReader.Close() })
	ids := make(map[int64]bool, n)
	for i := 0; i < n; i++ {
		record, err := outputReader.ReadMessage(ctx)
		if err != nil {
			t.Fatal(err)
		}
		msg, err := protocol.Unmarshal(record.Value)
		want := throughputInput(i)
		if err != nil || msg.RoomID != want.RoomID || msg.SenderID != want.SenderID || msg.ClientMsgID != want.ClientMsgID || msg.Content != "#" || msg.RoomSeq != int64(i/100+1) || msg.MessageID <= 0 || ids[msg.MessageID] {
			t.Fatalf("output mismatch at index=%d: message=%+v err=%v", i, msg, err)
		}
		ids[msg.MessageID] = true
	}
	conn, err := kafka.DialLeader(ctx, "tcp", brokers[0], output, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	last, err := conn.ReadLastOffset()
	if err != nil || last != int64(n) {
		t.Fatalf("expected exactly %d output records, end offset=%d err=%v", n, last, err)
	}
	t.Logf("VERIFIED count=%d unique_ids=%d room_sequences=contiguous input_order=preserved", n, len(ids))
}

type throughputReader struct {
	Reader
	limit                  int
	completed              atomic.Int64
	firstFetched           time.Time
	startup, fetch, commit time.Duration
}

func (r *throughputReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if r.completed.Load() >= int64(r.limit) {
		return kafka.Message{}, io.EOF
	}
	start := time.Now()
	msg, err := r.Reader.FetchMessage(ctx)
	if r.firstFetched.IsZero() {
		r.firstFetched = time.Now()
		r.startup = time.Since(start)
	} else {
		r.fetch += time.Since(start)
	}
	return msg, err
}

func (r *throughputReader) CommitMessages(ctx context.Context, messages ...kafka.Message) error {
	start := time.Now()
	err := r.Reader.CommitMessages(ctx, messages...)
	r.commit += time.Since(start)
	if err == nil {
		r.completed.Add(int64(len(messages)))
	}
	return err
}

func throughputInput(i int) protocol.MessageEnvelope {
	return protocol.MessageEnvelope{RoomID: int64(i%100 + 1), SenderID: int64(i%1000 + 1), ClientMsgID: fmt.Sprintf("perf-%d", i), Content: "#", Type: 1, CreatedAt: time.Unix(1750000000, 0)}
}

// Fixed-size runs avoid making the intentionally unbounded dedup map dominate
// benchmark calibration: -bench BenchmarkAssignUnique -benchtime=200000x -benchmem.
func BenchmarkAssignUnique(b *testing.B) {
	inputs := make([]protocol.MessageEnvelope, b.N)
	for i := range inputs {
		inputs[i] = throughputInput(i)
	}
	s := New()
	b.ReportAllocs()
	b.ResetTimer()
	for _, msg := range inputs {
		if _, err := s.Assign(msg); err != nil {
			b.Fatal(err)
		}
	}
}

// Kafka topic creation can precede leader metadata propagation. Retry only
// explicit UnknownTopicOrPartition responses (nothing accepted), not ambiguous writes.
func writeSeedBatch(ctx context.Context, writer *kafka.Writer, batch []kafka.Message) error {
	for {
		err := writer.WriteMessages(ctx, batch...)
		if err == nil {
			return nil
		}
		unknown := errors.Is(err, kafka.UnknownTopicOrPartition)
		var perMessage kafka.WriteErrors
		if errors.As(err, &perMessage) && len(perMessage) > 0 {
			unknown = true
			for _, item := range perMessage {
				if !errors.Is(item, kafka.UnknownTopicOrPartition) {
					unknown = false
					break
				}
			}
		}
		if !unknown {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func TestKafkaSequencerBatchSweep(t *testing.T) {
	if os.Getenv("KAFKA_SEQUENCER_TEST_BROKERS") == "" {
		t.Skip("explicit Kafka test broker required")
	}
	for _, size := range []int{0, 100, 500, 1000, 2000, 3000, 4000, 5000, 6000, 7000, 8000, 9000, 10000} {
		for round := 1; round <= 2; round++ {
			t.Run(fmt.Sprintf("batch_%05d/round_%d", size, round), func(t *testing.T) { runKafkaSequencerThroughput(t, size) })
		}
	}
}
