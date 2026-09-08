package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	messages "lan-im-go/services/messages/api"
	"lan-im-go/services/messages/runtime"
	"lan-im-go/services/messages/storage"
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
	module := messages.NewModule(deps.Messages, deps.Membership, deps.DB, storage.New())
	port := os.Getenv("MESSAGE_SERVER_PORT")
	if port == "" {
		port = "8083"
	}
	server := &http.Server{Addr: ":" + port, Handler: messages.NewRouter(module, deps.Ready),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("消息与文件 API 监听端口 %s", port)
	return runtime.Serve(ctx, server)
}
