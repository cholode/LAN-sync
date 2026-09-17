package producer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	protocol "lan-im-go/contracts/events"
)

type fakeMessageWriter struct {
	mu      sync.Mutex
	calls   [][]kafka.Message
	started chan struct{}
	gate    chan struct{}
	err     error
	closed  bool
	once    sync.Once
}

func TestHandleIncomingMessageAtPreservesGatewayArrivalTime(t *testing.T) {
	writer := &fakeMessageWriter{}
	client := newMessageClient(writer, "test", BatchOptions{
		MaxMessages: 1, MaxBytes: 4096, MaxWait: time.Second,
		QueueCapacity: 1, WriteTimeout: time.Second,
	})
	arrivedAt := time.Unix(1_800_000_000, 123_456_789)
	if err := client.HandleIncomingMessageAt(context.Background(), "7", 8, "#", "arrival-time", arrivedAt); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	writer.mu.Lock()
	written := writer.calls[0][0].Value
	writer.mu.Unlock()
	envelope, err := protocol.Unmarshal(written)
	if err != nil {
		t.Fatal(err)
	}
	if !envelope.CreatedAt.Equal(arrivedAt) {
		t.Fatalf("created_at = %v, want gateway arrival %v", envelope.CreatedAt, arrivedAt)
	}
}

func (w *fakeMessageWriter) WriteMessages(_ context.Context, messages ...kafka.Message) error {
	w.mu.Lock()
	w.calls = append(w.calls, append([]kafka.Message(nil), messages...))
	w.mu.Unlock()
	if w.started != nil {
		w.once.Do(func() { close(w.started) })
	}
	if w.gate != nil {
		<-w.gate
	}
	return w.err
}

func (w *fakeMessageWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return nil
}

func (w *fakeMessageWriter) batchSizes() []int {
	w.mu.Lock()
	defer w.mu.Unlock()
	result := make([]int, len(w.calls))
	for i := range w.calls {
		result[i] = len(w.calls[i])
	}
	return result
}

func sendTestMessage(ctx context.Context, client *MessageClient, id int) error {
	return client.HandleIncomingMessage(ctx, "1", id+1, "#", string(rune('a'+id)))
}

func TestMessageClientBatchesAndFlushesTail(t *testing.T) {
	writer := &fakeMessageWriter{}
	client := newMessageClient(writer, "test", BatchOptions{
		MaxMessages: 3, MaxBytes: 4096, MaxWait: 20 * time.Millisecond,
		QueueCapacity: 10, WriteTimeout: time.Second,
	})
	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) { defer wg.Done(); errs <- sendTestMessage(context.Background(), client, id) }(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	sizes := writer.batchSizes()
	if len(sizes) != 2 || sizes[0] != 3 || sizes[1] != 2 {
		t.Fatalf("unexpected batch sizes: %v", sizes)
	}
}

func TestMessageClientFlushesOnDeadline(t *testing.T) {
	writer := &fakeMessageWriter{}
	client := newMessageClient(writer, "test", BatchOptions{
		MaxMessages: 100, MaxBytes: 4096, MaxWait: 15 * time.Millisecond,
		QueueCapacity: 10, WriteTimeout: time.Second,
	})
	start := time.Now()
	if err := sendTestMessage(context.Background(), client, 1); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Fatalf("batch flushed before wait deadline: %v", elapsed)
	}
	_ = client.Close()
}

func TestMessageClientFullQueueFailsFast(t *testing.T) {
	writer := &fakeMessageWriter{started: make(chan struct{}), gate: make(chan struct{})}
	client := newMessageClient(writer, "test", BatchOptions{
		MaxMessages: 1, MaxBytes: 4096, MaxWait: time.Second,
		QueueCapacity: 1, WriteTimeout: time.Second,
	})
	first := make(chan error, 1)
	go func() { first <- sendTestMessage(context.Background(), client, 1) }()
	<-writer.started
	second := make(chan error, 1)
	go func() { second <- sendTestMessage(context.Background(), client, 2) }()
	deadline := time.Now().Add(time.Second)
	for len(client.queue) != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if err := sendTestMessage(context.Background(), client, 3); !errors.Is(err, ErrProducerQueueFull) {
		t.Fatalf("expected queue full, got %v", err)
	}
	close(writer.gate)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
}

func TestMessageClientBatchFailureAndClose(t *testing.T) {
	writeErr := errors.New("broker unavailable")
	writer := &fakeMessageWriter{err: writeErr}
	client := newMessageClient(writer, "test", BatchOptions{
		MaxMessages: 2, MaxBytes: 4096, MaxWait: 5 * time.Millisecond,
		QueueCapacity: 2, WriteTimeout: time.Second,
	})
	err := sendTestMessage(context.Background(), client, 1)
	if !errors.Is(err, writeErr) {
		t.Fatalf("expected write error, got %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sendTestMessage(context.Background(), client, 2); !errors.Is(err, ErrProducerClosed) {
		t.Fatalf("expected closed error, got %v", err)
	}
	if !writer.closed {
		t.Fatal("writer was not closed")
	}
}
