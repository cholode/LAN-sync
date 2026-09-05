package main

import (
	"context"
	"lan-im-go/config"
	"lan-im-go/infrastructure"
	"lan-im-go/pkg"
	"lan-im-go/repository"
	adminservice "lan-im-go/services/admin/application"
	"lan-im-go/services/admin/control"
	"lan-im-go/services/gateway/grpc"
	"lan-im-go/services/gateway/http"
	"lan-im-go/services/gateway/websocket"
	"lan-im-go/services/messages/api"
	"lan-im-go/services/messages/archiver"
	"lan-im-go/services/messages/search"
	"lan-im-go/services/messages/storage"
	roomapi "lan-im-go/services/rooms/api"
	roomservice "lan-im-go/services/rooms/application"
	"lan-im-go/shared/concurrency/taskpool"
	"lan-im-go/shared/observability/metrics"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"
)

// main 程序入口函数
func main() {
	// 微服务内部统一使用 UTC；北京时间只在展示层转换。
	time.Local = time.UTC

	// 独立启动指标与pprof管理服务
	if strings.ToLower(os.Getenv("METRICS_ENABLED")) != "false" {
		metricsAddr := os.Getenv("METRICS_ADDR")
		if metricsAddr == "" {
			metricsAddr = "0.0.0.0:6060"
		}
		metricsPath := os.Getenv("METRICS_PATH")
		if metricsPath == "" {
			metricsPath = "/metrics"
		}
		if !strings.HasPrefix(metricsPath, "/") {
			metricsPath = "/" + metricsPath
		}
		http.Handle(metricsPath, metrics.Handler())
		go func() {
			pkg.Infof("[系统启动] 指标与pprof管理服务监听 %s", metricsAddr)
			if err := http.ListenAndServe(metricsAddr, nil); err != nil {
				pkg.Fatalf("[致命错误] 指标服务启动失败: %v", err)
			}
		}()
	}

	// ================================
	// 阶段1：环境与基础设施初始化
	// ================================
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "root:123456@tcp(127.0.0.1:3306)/lan_im?charset=utf8mb4&parseTime=True&loc=UTC"
		pkg.Infoln("[警告] 未检测到DB_DSN环境变量，使用本地默认配置连接MySQL")
	}

	config.InitRedis()
	config.InitKafka()

	defer taskpool.Release()
	defer config.RedisClient.Close()
	defer config.KafkaProducer.Close()

	infrastructure.InitDatabase(dsn)
	messageRepo := messages.NewMySQLRepository(infrastructure.DB)
	if os.Getenv("MESSAGE_STORE") == "mongo" {
		infrastructure.InitMongo()
		defer infrastructure.CloseMongo()
		messageRepo = messages.NewMongoRepository(infrastructure.MessageCollection)
	}
	taskpool.Init(0) // 0 表示使用默认工作协程数
	if err := search.Init(context.Background()); err != nil {
		pkg.Fatalf("[Elasticsearch] init failed: %v", err)
	}
	defer search.Close()

	fileStorage := storage.New()

	// ================================
	// 阶段2：数据访问层初始化
	// ================================
	repository.InitRepositories(infrastructure.DB, messageRepo)
	pkg.Infoln("[系统就绪] 数据访问层(DAL)初始化完成")

	// ================================
	// 阶段3：Kafka 离线消息归档消费服务
	// ================================
	messagingCfg := config.Messaging()
	worker := archiver.NewWorker(
		messagingCfg.Kafka.Brokers,
		messagingCfg.Kafka.Topic,
		messagingCfg.Kafka.ArchiverGroup,
		config.RedisClient,
	)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		pkg.Infoln("[系统启动] Kafka离线消息归档协程进入循环监听...")
		worker.Start(ctx)
	}()

	// ================================
	// 阶段4：WebSocket 核心引擎启动
	// ================================
	hub := core.NewHub()
	metrics.RegisterTaskPoolMetrics(hub.FanoutPool())
	go hub.Run(ctx)
	go core.StartGlobalListener(ctx, hub)
	go core.StartRoomEventListener(ctx, hub)
	pkg.Infoln("[系统就绪] WebSocket核心引擎启动完成")

	// Python Agent 服务通过 Kafka 独立消费消息。Go 侧只保留其调用的 IMService。
	imGRPCAddr := os.Getenv("IM_GRPC_ADDR")
	if imGRPCAddr == "" {
		imGRPCAddr = "0.0.0.0:50052"
	}
	imSrv := imservice.NewServer(hub)
	go func() {
		if err := imSrv.Start(ctx, imGRPCAddr); err != nil {
			pkg.Fatalf("[致命错误] IMService gRPC 服务启动失败: %v", err)
		}
	}()

	localAdminRuntime := &admincontrol.LocalRuntimeController{
		Hub: hub,
	}
	// 独立管理端重新拆分时再开启远程控制面；合并部署默认走本地调用。
	if strings.EqualFold(os.Getenv("ADMIN_CONTROL_GRPC_ENABLED"), "true") {
		adminControlAddr := os.Getenv("ADMIN_CONTROL_GRPC_ADDR")
		if adminControlAddr == "" {
			adminControlAddr = "0.0.0.0:50053"
		}
		adminControlServer := admincontrol.NewServer(localAdminRuntime)
		go func() {
			if err := adminControlServer.Start(ctx, adminControlAddr, os.Getenv("ADMIN_CONTROL_TOKEN")); err != nil {
				pkg.Fatalf("[致命错误] AdminControl gRPC 服务启动失败: %v", err)
			}
		}()
	}

	// 主服务只保留运行所需的错误记录；管理 API 由独立 admin-service 提供。
	errorService := adminservice.NewErrorCenterService(infrastructure.DB)
	messageModule := messages.NewModule(messageRepo, repository.RoomMember, infrastructure.DB, fileStorage)
	roomModule := roomapi.NewModule(roomservice.NewService(repository.Room, repository.RoomMember, hub))

	// ================================
	// 阶段6：HTTP 服务与路由配置
	// ================================
	r := gateways.NewRouter(gateways.Dependencies{Hub: hub, DB: infrastructure.DB,
		Messages: messageModule, Rooms: roomModule, ErrorService: errorService, FrontendDir: "./frontend/dist"})

	// ================================
	// 阶段7：启动 HTTP 服务
	// ================================
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	srv := gateways.NewServer(":"+port, r)

	pkg.Infof("[系统启动] LAN-IM 服务端启动成功，监听端口 :%s", port)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		pkg.Fatalf("[致命错误] 服务启动失败: %v", err)
	}

	cancel()
}
