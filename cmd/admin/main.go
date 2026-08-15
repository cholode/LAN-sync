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
	adminservice "lan-im-go/internal/admin"
	"lan-im-go/internal/admincontrol"
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
	messageRepo := repository.NewMessageRepoImpl(infrastructure.DB)
	if os.Getenv("MESSAGE_STORE") == "mongo" {
		infrastructure.InitMongo()
		defer infrastructure.CloseMongo()
		messageRepo = repository.NewMongoMessageRepo(infrastructure.MessageCollection)
	}
	repository.InitRepositories(infrastructure.DB, messageRepo)
	adminapi.InitFileStorage()

	controlBaseURL := os.Getenv("IM_CONTROL_BASE_URL")
	if controlBaseURL == "" {
		controlBaseURL = "http://127.0.0.1:8080"
	}
	runtimeClient := admincontrol.NewHTTPClient(controlBaseURL, os.Getenv("ADMIN_CONTROL_TOKEN"))
	adminapi.InitRuntimeController(runtimeClient)

	adminAuditService := adminservice.NewAuditService(infrastructure.DB)
	adminMessageStore := adminservice.NewMessageStatsStore(infrastructure.DB, infrastructure.MessageCollection, os.Getenv("MESSAGE_STORE"))
	ragService := adminservice.NewRAGService(infrastructure.DB)
	moderationService := adminservice.NewModerationService(infrastructure.DB, adminAuditService)
	adminHealthService := adminservice.NewHealthService(infrastructure.DB, config.RedisClient, adminapi.Storage, runtimeClient)
	dashboardService := adminservice.NewDashboardService(
		infrastructure.DB,
		infrastructure.MessageCollection,
		os.Getenv("MESSAGE_STORE"),
		runtimeClient,
		moderationService,
		ragService,
		adminHealthService,
	)
	adminErrorService := adminservice.NewErrorCenterService(infrastructure.DB)

	adminapi.InitAdminDashboardService(dashboardService)
	adminapi.InitAdminRAGService(ragService)
	adminapi.InitAdminModerationService(moderationService)
	adminapi.InitAdminHealthService(adminHealthService)
	adminapi.InitAdminUserService(adminservice.NewUserService(infrastructure.DB, adminMessageStore, runtimeClient, adminAuditService))
	adminapi.InitAdminConnectionService(adminservice.NewConnectionService(runtimeClient, adminAuditService))
	adminapi.InitAdminFileServiceVar(adminservice.NewFileService(infrastructure.DB, adminapi.Storage, adminAuditService))
	adminapi.InitAdminAgentConfigService(adminservice.NewAgentConfigService(infrastructure.DB, adminAuditService))
	adminapi.InitAdminToolCallService(adminservice.NewToolCallService(infrastructure.DB))
	adminapi.InitAdminErrorService(adminErrorService)
	adminapi.InitAdminAuditService(adminAuditService)
	adminapi.InitAdminAlertService(adminservice.NewAlertService(infrastructure.DB, adminHealthService, runtimeClient))
	adminapi.InitAdminRoomService(adminservice.NewRoomService(infrastructure.DB, adminMessageStore, runtimeClient, adminAuditService))

	router := gin.New()
	router.Use(middleware.RequestID())
	router.Use(middleware.RecoveryWithErrorRecorder(adminErrorService))
	router.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	admin := router.Group("/api/v1/admin")
	admin.Use(middleware.JWTAuth(), middleware.RequireAdmin(), middleware.AdminRateLimit(10, 30))
	adminapi.RegisterAdminRoutes(admin)

	router.GET("/admin", func(c *gin.Context) {
		c.File("./admin-service/web/dist/admin.html")
	})
	router.GET("/admin/*path", func(c *gin.Context) {
		c.File("./admin-service/web/dist/admin.html")
	})

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
