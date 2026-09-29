package producer_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"lan-im-go/services/messages/producer"
	"lan-im-go/services/messages/sequencer"
)

func TestKafkaBatchProducerThroughput(t *testing.T) {
	raw := os.Getenv("KAFKA_PRODUCER_TEST_BROKERS")
	if raw == "" {
		t.Skip("set KAFKA_PRODUCER_TEST_BROKERS to enable the Kafka integration test")
	}
	brokers := strings.Split(raw, ",")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	topic := fmt.Sprintf("producer_batch_%d", time.Now().UnixNano())
	if err := sequencer.EnsureTopics(ctx, brokers, topic); err != nil {
		t.Fatal(err)
	}
	client := producer.NewMessageClient(brokers, topic, producer.BatchOptions{
		MaxMessages: 1000, MaxBytes: 1 << 20, MaxWait: 5 * time.Millisecond,
		QueueCapacity: 20000, WriteTimeout: 10 * time.Second,
	})
	defer client.Close()

	const senders, perSender = 1000, 10
	start := time.Now()
	errs := make(chan error, senders)
	var wg sync.WaitGroup
	for sender := 0; sender < senders; sender++ {
		wg.Add(1)
		go func(sender int) {
			defer wg.Done()
			for sequence := 0; sequence < perSender; sequence++ {
				if err := client.HandleIncomingMessage(ctx, fmt.Sprint(sender%100+1), sender+1, "#", fmt.Sprintf("batch-%d-%d", sender, sequence)); err != nil {
					errs <- err
					return
				}
			}
		}(sender)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	conn, err := kafka.DialLeader(ctx, "tcp", brokers[0], topic, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	last, err := conn.ReadLastOffset()
	if err != nil || last != senders*perSender {
		t.Fatalf("expected %d records, end offset=%d err=%v", senders*perSender, last, err)
	}
	t.Logf("messages=%d seconds=%.3f rate=%.0f/s", senders*perSender, elapsed.Seconds(), float64(senders*perSender)/elapsed.Seconds())
}
