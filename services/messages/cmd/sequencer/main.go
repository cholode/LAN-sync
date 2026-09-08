package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
	"lan-im-go/config"
	protocol "lan-im-go/contracts/events"
	"lan-im-go/services/messages/runtime"
	"lan-im-go/services/messages/sequencer"
	"lan-im-go/shared/observability/metrics"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	cfg := config.Messaging().Kafka
	if cfg.Topic == cfg.IngressTopic {
		return fmt.Errorf("待处理主题和正式主题不能相同")
	}
	startup, stop := context.WithTimeout(ctx, 30*time.Second)
	err := sequencer.Retry(startup, func() error { return sequencer.EnsureTopics(startup, cfg.Brokers, cfg.IngressTopic, cfg.Topic) })
	stop()
	if err != nil {
		return err
	}
	config.InitRedis()
	defer config.RedisClient.Close()
	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: cfg.Brokers, Topic: cfg.IngressTopic, GroupID: "im_sequencer_" + cfg.IngressTopic, MinBytes: 1, MaxBytes: 10e6, MaxWait: 20 * time.Millisecond, CommitInterval: 0})
	defer reader.Close()
	relay := kafka.NewReader(kafka.ReaderConfig{Brokers: cfg.Brokers, Topic: cfg.Topic, GroupID: "im_relay_" + cfg.Topic, MinBytes: 1, MaxBytes: 10e6, MaxWait: 20 * time.Millisecond, CommitInterval: 0})
	defer relay.Close()
	writer := &kafka.Writer{Addr: kafka.TCP(cfg.Brokers...), Topic: cfg.Topic, Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll, BatchTimeout: 5 * time.Millisecond}
	defer writer.Close()
	done := make(chan error, 3)
	go func() {
		done <- sequencer.Run(ctx, reader, func(ctx context.Context, msg kafka.Message) error { return writer.WriteMessages(ctx, msg) }, sequencer.New())
	}()
	// 广播循环独立于编号与数据库归档，Redis 故障不会阻止编号结果写入 Kafka。
	go func() {
		done <- sequencer.Relay(ctx, relay, func(ctx context.Context, msg protocol.MessageEnvelope, value []byte) error {
			channel := fmt.Sprintf("im:broadcast:room:%d", msg.RoomID)
			err := config.RedisClient.Publish(ctx, channel, value).Err()
			metrics.ObserveRedisPubSub(channel, "publish", err)
			return err
		})
	}()
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		check, stop := context.WithTimeout(r.Context(), 2*time.Second)
		defer stop()
		if config.RedisClient.Ping(check).Err() != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	addr := os.Getenv("METRICS_ADDR")
	if addr == "" {
		addr = ":6062"
	}
	go func() {
		done <- runtime.Serve(ctx, &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second})
	}()
	log.Print("编号器已启动：单实例、内存状态，不支持重启恢复；广播读取正式消息")
	first := <-done
	cancel()
	for i := 0; i < 2; i++ {
		<-done
	}
	if errors.Is(first, context.Canceled) {
		return nil
	}
	return first
}
