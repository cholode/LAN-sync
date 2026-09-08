package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lan-im-go/config"
	"lan-im-go/services/messages/archiver"
	"lan-im-go/services/messages/runtime"
	"lan-im-go/shared/observability/metrics"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	time.Local = time.UTC
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	deps, err := runtime.Open(ctx)
	if err != nil {
		return err
	}
	defer deps.Close()
	cfg := config.Messaging().Kafka
	worker, err := archiver.NewWorker(cfg.Brokers, cfg.Topic, cfg.ArchiverGroup, config.RedisClient, deps.Messages)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		checkCtx, stop := context.WithTimeout(r.Context(), 2*time.Second)
		defer stop()
		if deps.Ready(checkCtx) != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	addr := os.Getenv("METRICS_ADDR")
	if addr == "" {
		addr = ":6061"
	}
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	serverDone := make(chan error, 1)
	go func() { serverDone <- runtime.Serve(ctx, srv); cancel() }()
	// 归档必须先退出，再关闭 Redis、数据库和索引客户端。
	workerErr := worker.Start(ctx)
	cancel()
	serverErr := <-serverDone
	return errors.Join(workerErr, serverErr)
}
