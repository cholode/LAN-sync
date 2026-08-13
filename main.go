package main

import (
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"lan-im-go/agent"
	"lan-im-go/agent/llm"
	"lan-im-go/api"
	"lan-im-go/config"
	"lan-im-go/core"
	"lan-im-go/infrastructure"
	"lan-im-go/internal/archiver"
	"lan-im-go/internal/taskpool"
	"lan-im-go/middleware"
	"lan-im-go/pkg"
	"lan-im-go/repository"
	"net/http"
	"os"
	"time"
)

// main 程序入口函数
func main() {
	// 独立启动pprof性能分析服务
	go func() {
		err := http.ListenAndServe("0.0.0.0:6060", nil)
		if err != nil {
			panic("pprof start failed: " + err.Error())
		}
	}()

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
	taskpool.Init(0) // 0=默认线程数(CPU*2)
	api.InitFileStorage()

	// ================================
	// 阶段2：数据访问层初始化
	// ================================
	repository.InitRepositories(infrastructure.DB)
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
	pkg.Infoln("[系统就绪] WebSocket核心引擎启动完成")

	// ================================
	// 阶段5：Agent 管理系统启动
	// ================================
	llmClient := llm.NewClient()
	agentMgr := agent.NewAgentManager(infrastructure.DB, llmClient, hub)
	go agentMgr.Start(ctx)

	// ================================
	// 阶段6：HTTP 服务与路由配置
	// ================================
	gin.SetMode(gin.DebugMode)

	r := gin.New()
	r.Use(gin.Recovery())

	pprof.Register(r)

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

		authorized.GET("/rooms/:id/messages", api.GetChatHistory())
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
	admin.Use(middleware.JWTAuth(), middleware.SuperAdminOnly())
	{
		admin.DELETE("/users/:id", api.AdminDeleteUser(hub))
		admin.DELETE("/rooms/:id", api.AdminDeleteRoom(hub))
	}

	// ================================
	// 静态文件服务 - 前端 SPA
	// ================================
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
