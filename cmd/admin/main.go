package main

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	adminapi "lan-im-go/admin-service/api"
	"lan-im-go/config"
	"lan-im-go/infrastructure"
	"lan-im-go/internal/admincontrol"
	"lan-im-go/messages"
	"lan-im-go/middleware"
	"lan-im-go/pkg"
	"lan-im-go/repository"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "root:123456@tcp(127.0.0.1:3306)/lan_im?charset=utf8mb4&parseTime=True&loc=Local"
	}

	config.InitRedis()
	defer config.RedisClient.Close()

	infrastructure.InitDatabase(dsn)
	messageRepo := messages.NewMySQLRepository(infrastructure.DB)
	if os.Getenv("MESSAGE_STORE") == "mongo" {
		infrastructure.InitMongo()
		defer infrastructure.CloseMongo()
		messageRepo = messages.NewMongoRepository(infrastructure.MessageCollection)
	}
	repository.InitRepositories(infrastructure.DB, messageRepo)
	adminapi.InitFileStorage()

	controlAddr := os.Getenv("ADMIN_CONTROL_GRPC_ADDR")
	if controlAddr == "" {
		controlAddr = "127.0.0.1:50053"
	}
	runtimeClient, err := admincontrol.NewGRPCClient(controlAddr, os.Getenv("ADMIN_CONTROL_TOKEN"))
	if err != nil {
		pkg.Fatalf("创建 AdminControl gRPC 客户端失败: %v", err)
	}
	defer runtimeClient.Close()
	adminModule := adminapi.NewModule(adminapi.ModuleDependencies{
		DB:                infrastructure.DB,
		Redis:             config.RedisClient,
		MessageCollection: infrastructure.MessageCollection,
		MessageStore:      os.Getenv("MESSAGE_STORE"),
		Storage:           adminapi.Storage,
		Runtime:           runtimeClient,
	})

	router := gin.New()
	router.Use(middleware.RequestID())
	router.Use(middleware.RecoveryWithErrorRecorder(adminModule.ErrorService))
	router.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	adminModule.RegisterRoutes(router)

	port := os.Getenv("ADMIN_HTTP_PORT")
	if port == "" {
		port = "8081"
	}
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       15 * time.Second,
	}

	pkg.Infof("管理端服务启动成功，监听端口 :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		pkg.Fatalf("管理端服务启动失败: %v", err)
	}
}
