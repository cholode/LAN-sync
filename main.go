package main

import (
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	//"io"
	"lan-im-go/api"
	"lan-im-go/config"
	"lan-im-go/core"
	"lan-im-go/infrastructure"
	"lan-im-go/internal/archiver"
	"lan-im-go/middleware"
	"lan-im-go/repository"
	"log"
	"net/http"
	"os"
	"time"
)

// main 程序入口函数
// 按顺序执行：基础设施初始化 → 数据访问层初始化 → 后台消费服务启动 → WebSocket引擎启动 → HTTP服务启动
func main() {
	// 独立启动pprof性能分析服务，监听6060端口，不阻塞主业务流程
	go func() {
		// 绑定0.0.0.0允许外部/宿主机访问，生产环境建议限制为内网地址
		err := http.ListenAndServe("0.0.0.0:6060", nil)
		if err != nil {
			panic("pprof start failed: " + err.Error())
		}
	}()

	//log.SetOutput(io.Discard)

	// ================================
	// 阶段1：环境与基础设施初始化
	// ================================
	// 从环境变量读取数据库DSN，为空时使用本地默认配置（仅适用于开发调试）
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "root:123456@tcp(127.0.0.1:3306)/lan_im?charset=utf8mb4&parseTime=True&loc=Local"
		log.Println("[警告] 未检测到DB_DSN环境变量，使用本地默认配置连接MySQL")
	}

	// 初始化中间件：Redis缓存、Kafka消息队列
	config.InitRedis()
	config.InitKafka()

	// 程序退出时优雅关闭资源连接
	defer config.RedisClient.Close()
	defer config.KafkaProducer.Close()

	// 初始化数据库连接池并自动同步表结构
	// 数据库连接失败时直接panic终止程序，保证服务启动的完整性
	infrastructure.InitDatabase(dsn)
	api.InitFileDirs()

	// ================================
	// 阶段2：数据访问层(DAL)初始化
	// ================================
	// 注入数据库连接实例到数据访问层
	// 所有业务逻辑统一通过数据访问层接口操作数据库
	repository.InitRepositories(infrastructure.DB)
	log.Println("[系统就绪] 数据访问层(DAL)初始化完成")

	repository.InitRepositories(infrastructure.DB)
	log.Println("[系统就绪] 数据访问层(DAL) 初始化完成")

	// ================================
	// 阶段3：Kafka离线消息归档消费服务启动
	// ================================
	// 从环境变量读取Kafka地址，为空时使用本地默认地址
	kafkaAddrStr := os.Getenv("KAFKA_ADDR")
	if kafkaAddrStr == "" {
		kafkaAddrStr = "127.0.0.1:9092"
		log.Println("[警告] 未检测到KAFKA_ADDR环境变量，使用本地默认配置连接Kafka")
	}

	// 初始化消息归档Worker，内部自动调用repository持久化消息到MySQL
	worker := archiver.NewWorker([]string{kafkaAddrStr}, "im_chat_messages", "mysql_archiver_group", config.RedisClient)

	// 创建全局根上下文，用于控制所有后台协程的生命周期
	ctx, cancel := context.WithCancel(context.Background())

	// 启动Kafka消费协程，阻塞监听消息队列
	go func() {
		log.Println("[系统启动] Kafka离线消息归档协程进入循环监听...")
		worker.Start(ctx)
	}()

	// ================================
	// 阶段4：WebSocket核心引擎启动
	// ================================
	// 创建全局WebSocket连接管理器Hub
	hub := core.NewHub()
	// 启动Hub主循环，处理连接注册、注销和消息广播
	go hub.Run(ctx)
	// 启动全局消息监听器，接收Kafka消息并转发到对应房间
	go core.StartGlobalListener(ctx, hub)
	log.Println("[系统就绪] WebSocket核心引擎启动完成")

	// ================================
	// 阶段5：HTTP服务与路由配置
	// ================================
	// 设置Gin运行模式，生产环境必须切换为ReleaseMode
	//gin.SetMode(gin.ReleaseMode)
	gin.SetMode(gin.DebugMode)

	// 自定义Gin引擎，禁用默认日志中间件，保留崩溃恢复中间件
	r := gin.New()
	//r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// 注册pprof性能分析路由
	pprof.Register(r)

	// ================================
	// 跨域资源共享(CORS)配置
	// ================================
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true, // 开发环境允许所有来源，生产环境必须配置指定前端域名
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour, // 预检请求缓存时间，减少重复请求
	}))

	// ================================
	// 路由分组配置
	// ================================

	// 公开路由组：无需身份认证即可访问
	public := r.Group("/api/v1")
	{
		// 用户认证接口
		public.POST("/register", api.RegisterHandler)
		public.POST("/login", api.LoginHandler)
		// 文件下载接口：路径包含文件SHA-256哈希作为能力链，无需JWT即可直接下载
		public.GET("/download/*filepath", api.DownloadFile)
	}

	// 授权路由组：需要JWT身份认证才能访问
	authorized := r.Group("/api/v1")
	// 注册JWT身份认证中间件
	authorized.Use(middleware.JWTAuth())
	{
		// WebSocket连接建立端点
		log.Printf("进入WebSocket连接配置\n")
		authorized.GET("/ws", func(c *gin.Context) {
			api.WsEndpoint(hub)(c)
		})

		// 分片文件上传接口
		authorized.GET("/upload/status", api.CheckUploadStatus)
		authorized.POST("/upload/chunk", api.UploadChunk)
		authorized.POST("/upload/merge", api.MergeChunks)
		authorized.DELETE("/upload/cancel", api.CancelUpload)

		// 群聊相关接口
		authorized.GET("/rooms/:id/messages", api.GetChatHistory())
		authorized.POST("/rooms/:id/join", api.JoinRoom(hub))
		authorized.GET("/rooms/:id/members", api.GetRoomMembers())
		authorized.DELETE("/rooms/:id/members/:user_id", api.RemoveRoomMember(hub))
		authorized.DELETE("/rooms/:id/disband", api.OwnerDisbandRoom(hub))

		// 群聊管理接口
		authorized.POST("/rooms", api.CreateRoom(hub))
		authorized.GET("/my_rooms", api.GetMyRooms())
	}

	// 管理员路由组：需要超级管理员权限
	admin := r.Group("/api/v1/admin")
	// 中间件执行顺序：先身份认证，再权限校验
	admin.Use(middleware.JWTAuth(), middleware.SuperAdminOnly())
	{
		// 管理员操作接口
		admin.DELETE("/users/:id", api.AdminDeleteUser(hub))
		admin.DELETE("/rooms/:id", api.AdminDeleteRoom(hub))
	}

	// ================================
	// 阶段6：启动HTTP服务
	// ================================
	// 从环境变量读取服务端口，为空时默认使用8080
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	// 配置HTTP服务器参数
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,                // 将Gin路由引擎挂载到底层HTTP服务器
		ReadTimeout:       5 * time.Second,  // 读取完整请求头和请求体的最长时间
		ReadHeaderTimeout: 3 * time.Second,  // 仅读取请求头的最长时间，防止Slowloris攻击
		WriteTimeout:      10 * time.Second, // 写入响应的最长时间
		IdleTimeout:       15 * time.Second, // TCP Keep-Alive连接的最长空闲时间
	}

	log.Printf("[系统启动] LAN-IM 服务端启动成功，监听端口 :%s", port)

	// 启动HTTP服务，阻塞等待请求
	// 只有服务异常关闭或主动调用Shutdown时才会返回
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[致命错误] 服务启动失败: %v", err)
	}

	// 收到关闭信号后，取消全局上下文，通知所有后台协程优雅退出
	cancel()
}
