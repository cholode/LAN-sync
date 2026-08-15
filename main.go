package main

import (
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"lan-im-go/agent"
	"lan-im-go/api"
	"lan-im-go/config"
	"lan-im-go/core"
	"lan-im-go/infrastructure"
	adminservice "lan-im-go/internal/admin"
	"lan-im-go/internal/agentclient"
	"lan-im-go/internal/archiver"
	"lan-im-go/internal/imservice"
	"lan-im-go/internal/metrics"
	"lan-im-go/internal/search"
	"lan-im-go/internal/taskpool"
	"lan-im-go/middleware"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"lan-im-go/repository"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"
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
	messageRepo := repository.NewMessageRepoImpl(infrastructure.DB)
	if os.Getenv("MESSAGE_STORE") == "mongo" {
		infrastructure.InitMongo()
		defer infrastructure.CloseMongo()
		messageRepo = repository.NewMongoMessageRepo(infrastructure.MessageCollection)
	}
	taskpool.Init(0) // 0=default workers(CPU*2)
	metrics.RegisterTaskPoolMetrics()
	if err := search.Init(context.Background()); err != nil {
		pkg.Fatalf("[Elasticsearch] init failed: %v", err)
	}
	defer search.Close()

	api.InitFileStorage()

	// ================================
	// 阶段2：数据访问层初始化
	// ================================
	repository.InitRepositories(infrastructure.DB, messageRepo)
	pkg.Infoln("[系统就绪] 数据访问层(DAL)初始化完成")

	// ================================
	// 阶段3：Kafka 离线消息归档消费服务
	// ================================
	kafkaAddrStr := os.Getenv("KAFKA_ADDR")
	if kafkaAddrStr == "" {
		kafkaAddrStr = "127.0.0.1:9092"
		pkg.Infoln("[警告] 未检测到KAFKA_ADDR环境变量，使用本地默认配置连接Kafka")
	}

	worker := archiver.NewWorker([]string{kafkaAddrStr}, "im_chat_messages", "mysql_archiver_group", config.RedisClient)

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
	api.InitAdminDashboardService(adminservice.NewDashboardService(
		infrastructure.DB,
		infrastructure.MessageCollection,
		os.Getenv("MESSAGE_STORE"),
		hub,
	))
	api.InitAdminRAGService(adminservice.NewRAGService(infrastructure.DB))
	adminAuditService := adminservice.NewAuditService(infrastructure.DB)
	adminMessageStore := adminservice.NewMessageStatsStore(infrastructure.DB, infrastructure.MessageCollection, os.Getenv("MESSAGE_STORE"))
	api.InitAdminUserService(adminservice.NewUserService(infrastructure.DB, adminMessageStore, hub, adminAuditService))
	api.InitAdminConnectionService(adminservice.NewConnectionService(hub, adminAuditService))
	api.InitAdminModerationService(adminservice.NewModerationService(infrastructure.DB, adminAuditService))
	api.InitAdminFileService(adminservice.NewFileService(infrastructure.DB, api.Storage, adminAuditService))
	api.InitAdminAgentConfigService(adminservice.NewAgentConfigService(infrastructure.DB, adminAuditService))
	api.InitAdminToolCallService(adminservice.NewToolCallService(infrastructure.DB))
	adminErrorService := adminservice.NewErrorCenterService(infrastructure.DB)
	api.InitAdminErrorService(adminErrorService)
	api.InitAdminAuditService(adminAuditService)
	adminHealthService := adminservice.NewHealthService(infrastructure.DB, config.RedisClient, api.Storage, hub)
	api.InitAdminHealthService(adminHealthService)
	api.InitAdminAlertService(adminservice.NewAlertService(infrastructure.DB, adminHealthService))
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
	api.InitAdminRoomService(adminservice.NewRoomService(infrastructure.DB, adminMessageStore, hub, agentMgr, adminAuditService))
	go agentMgr.Start(ctx)

	// ================================
	// 阶段6：HTTP 服务与路由配置
	// ================================
	gin.SetMode(gin.DebugMode)

	r := gin.New()
	r.Use(middleware.RecoveryWithErrorRecorder(adminErrorService))
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		metrics.ObserveAPIRequest(c.Writer.Status(), time.Since(start))
	})

	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 公开路由
	public := r.Group("/api/v1")
	{
		public.POST("/register", api.RegisterHandler)
		public.POST("/login", api.LoginHandler)
		public.GET("/download/*filepath", api.DownloadFile)
	}

	// 授权路由
	authorized := r.Group("/api/v1")
	authorized.Use(middleware.JWTAuth())
	{
		pkg.Infof("进入WebSocket连接配置\n")
		authorized.GET("/ws", func(c *gin.Context) {
			api.WsEndpoint(hub)(c)
		})

		authorized.POST("/files/presign", api.PreSignUploadHandler)
		authorized.POST("/files/complete", api.CompleteUploadHandler)

		authorized.GET("/rooms/:id/messages", api.GetChatHistory())
		authorized.GET("/rooms/:id/messages/search", api.SearchMessages())
		authorized.POST("/rooms/:id/join", api.JoinRoom(hub))
		authorized.GET("/rooms/:id/members", api.GetRoomMembers())
		authorized.DELETE("/rooms/:id/members/:user_id", api.RemoveRoomMember(hub))
		authorized.DELETE("/rooms/:id/disband", api.OwnerDisbandRoom(hub))

		authorized.POST("/rooms", api.CreateRoom(hub))
		authorized.GET("/my_rooms", api.GetMyRooms())

		// Agent 管理路由
		api.RegisterAgentRoutes(authorized, agentMgr, infrastructure.DB)
	}

	// 管理员路由
	admin := r.Group("/api/v1/admin")
	admin.Use(middleware.JWTAuth(), middleware.RequireAdmin())
	{
		admin.GET("/dashboard/runtime", middleware.RequirePermission(models.PermDashboardRead), api.AdminDashboardRuntime)
		admin.GET("/dashboard/message-traffic", middleware.RequirePermission(models.PermDashboardRead), api.AdminDashboardMessageTraffic)
		admin.GET("/dashboard/agent", middleware.RequirePermission(models.PermAgentRead), api.AdminAgentDashboard)
		admin.GET("/dashboard/rag", middleware.RequireAnyPermission(models.PermDashboardRead, models.PermAgentRead), api.AdminRAGDashboard)
		admin.GET("/rag/queries", middleware.RequirePermission(models.PermAgentRead), api.AdminRAGQueries)
		admin.GET("/dashboard/moderation", middleware.RequirePermission(models.PermModerationRead), api.AdminModerationDashboard)
		admin.GET("/moderation", middleware.RequirePermission(models.PermModerationRead), api.AdminModerationList)
		admin.GET("/moderation/:id", middleware.RequirePermission(models.PermModerationRead), api.AdminModerationDetail)
		admin.POST("/moderation/:id/action", middleware.RequirePermission(models.PermModerationReview), api.AdminModerationAction)
		admin.GET("/dashboard/overview", middleware.RequirePermission(models.PermDashboardRead), api.AdminDashboardOverview)
		admin.GET("/users", middleware.RequirePermission(models.PermUserRead), api.AdminUserList)
		admin.GET("/users/:id", middleware.RequirePermission(models.PermUserRead), api.AdminUserDetail)
		admin.POST("/users/:id/action", middleware.RequireAnyPermission(models.PermUserBan, models.PermUserKick, models.PermUserRoleUpdate), api.AdminUserAction)
		admin.DELETE("/users/:id", middleware.RequirePermission(models.PermUserDelete), api.AdminDeleteUser(hub))
		admin.GET("/rooms", middleware.RequirePermission(models.PermRoomRead), api.AdminRoomList)
		admin.GET("/rooms/:id", middleware.RequirePermission(models.PermRoomRead), api.AdminRoomDetail)
		admin.POST("/rooms/:id/action", middleware.RequireAnyPermission(models.PermRoomFreeze, models.PermRoomDelete, models.PermAgentConfig), api.AdminRoomAction)
		admin.GET("/connections", middleware.RequirePermission(models.PermConnectionRead), api.AdminConnectionList)
		admin.POST("/connections/close", middleware.RequirePermission(models.PermConnectionClose), api.AdminConnectionClose)
		admin.POST("/connections/force-offline", middleware.RequirePermission(models.PermConnectionClose), api.AdminUserForceOffline)
		admin.GET("/files", middleware.RequirePermission(models.PermFileRead), api.AdminFileList)
		admin.GET("/files/scan", middleware.RequirePermission(models.PermFileRead), api.AdminFileScan)
		admin.POST("/files/cleanup", middleware.RequirePermission(models.PermFileDelete), api.AdminFileCleanup)
		admin.GET("/files/:id", middleware.RequirePermission(models.PermFileRead), api.AdminFileDetail)
		admin.GET("/files/:id/download", middleware.RequirePermission(models.PermFileRead), api.AdminFileDownload)
		admin.DELETE("/files/:id", middleware.RequirePermission(models.PermFileDelete), api.AdminFileDelete)
		admin.GET("/agent-config", middleware.RequirePermission(models.PermAgentRead), api.AdminAgentConfigGet)
		admin.GET("/agent-config/history", middleware.RequirePermission(models.PermAgentRead), api.AdminAgentConfigHistory)
		admin.PUT("/agent-config", middleware.RequirePermission(models.PermAgentConfig), api.AdminAgentConfigUpdate)
		admin.POST("/agent-config/rollback", middleware.RequirePermission(models.PermAgentConfig), api.AdminAgentConfigRollback)
		admin.GET("/tool-calls", middleware.RequirePermission(models.PermAgentRead), api.AdminToolCallList)
		admin.GET("/errors", middleware.RequirePermission(models.PermSystemRead), api.AdminErrorList)
		admin.POST("/errors/:id/resolve", middleware.RequirePermission(models.PermSystemRead), api.AdminErrorResolve)
		admin.GET("/audit-logs", middleware.RequirePermission(models.PermAuditRead), api.AdminAuditList)
		admin.GET("/health", middleware.RequirePermission(models.PermSystemRead), api.AdminHealthCheck)
		admin.GET("/alerts", middleware.RequirePermission(models.PermSystemRead), api.AdminAlertList)
		admin.GET("/alerts/unresolved-count", middleware.RequireAnyPermission(models.PermDashboardRead, models.PermSystemRead), api.AdminAlertUnresolvedCount)
		admin.POST("/alerts/evaluate", middleware.RequirePermission(models.PermSystemRead), api.AdminAlertEvaluate)
		admin.POST("/alerts/:id/resolve", middleware.RequirePermission(models.PermSystemRead), api.AdminAlertResolve)
		admin.DELETE("/rooms/:id", middleware.RequirePermission(models.PermRoomDelete), api.AdminDeleteRoom(hub))
	}

	// ================================
	// 静态文件服务 - 前端 SPA
	// ================================
	// ??????????????
	r.GET("/admin", func(c *gin.Context) {
		c.File("./frontend/dist/admin.html")
	})
	r.GET("/admin/*path", func(c *gin.Context) {
		c.File("./frontend/dist/admin.html")
	})

	r.Static("/assets", "./frontend/dist/assets")
	r.GET("/", func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})
	r.NoRoute(func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})

	// ================================
	// 阶段7：启动 HTTP 服务
	// ================================
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       15 * time.Second,
	}

	pkg.Infof("[系统启动] LAN-IM 服务端启动成功，监听端口 :%s", port)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		pkg.Fatalf("[致命错误] 服务启动失败: %v", err)
	}

	cancel()
}
