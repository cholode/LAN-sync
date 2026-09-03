package main

import (
	"context"
	"lan-im-go/config"
	"lan-im-go/infrastructure"
	"lan-im-go/pkg"
	"lan-im-go/repository"
	adminservice "lan-im-go/services/admin/application"
	"lan-im-go/services/admin/control"
	"lan-im-go/services/agent/application"
	"lan-im-go/services/files/api"
	"lan-im-go/services/files/api/storage"
	"lan-im-go/services/gateway/clients"
	"lan-im-go/services/gateway/grpc"
	"lan-im-go/services/gateway/http"
	"lan-im-go/services/gateway/websocket"
	"lan-im-go/services/messages/api"
	"lan-im-go/services/messages/archiver"
	"lan-im-go/services/messages/search"
	"lan-im-go/shared/concurrency/taskpool"
	"lan-im-go/shared/observability/metrics"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
)

// main 程序入口函数
func main() {
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
		dsn = "root:123456@tcp(127.0.0.1:3306)/lan_im?charset=utf8mb4&parseTime=True&loc=Local"
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
	taskpool.Init(0) // 0=default workers(CPU*2)
	metrics.RegisterTaskPoolMetrics()
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
	go hub.Run(ctx)
	go core.StartGlobalListener(ctx, hub)
	pkg.Infoln("[系统就绪] WebSocket核心引擎启动完成")

	// ================================
	// 阶段5：Agent 管理系统启动
	// ================================
	agentAddr := os.Getenv("AGENT_GRPC_ADDR")
	if agentAddr == "" {
		agentAddr = "127.0.0.1:50051"
	}
	agentClient, err := agentclient.New(agentAddr)
	if err != nil {
		pkg.Fatalf("[致命错误] 创建 Agent gRPC 客户端失败: %v", err)
	}
	defer agentClient.Close()

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

	agentMgr := agent.NewAgentManager(infrastructure.DB, agentClient, hub)
	go agentMgr.Start(ctx)

	localAdminRuntime := &admincontrol.LocalRuntimeController{
		Hub:          hub,
		AgentManager: agentMgr,
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

	// 主服务只保留运行所需的错误记录和上传元数据能力；管理 API 由独立 admin-service 提供。
	auditService := adminservice.NewAuditService(infrastructure.DB)
	errorService := adminservice.NewErrorCenterService(infrastructure.DB)
	fileService := adminservice.NewFileService(infrastructure.DB, fileStorage, auditService)
	fileModule := files.NewModule(fileStorage, fileService)
	messageModule := messages.NewModule(messageRepo, repository.RoomMember)

	// ================================
	// 阶段6：HTTP 服务与路由配置
	// ================================
	r := gateways.NewRouter(gateways.Dependencies{Hub: hub, Agent: agentMgr, DB: infrastructure.DB,
		Files: fileModule, Messages: messageModule, ErrorService: errorService, FrontendDir: "./frontend/dist"})

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
