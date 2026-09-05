package main

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"lan-im-go/config"
	"lan-im-go/pkg"
	"lan-im-go/repository"
	roomapi "lan-im-go/services/rooms/api"
	"lan-im-go/services/rooms/application"
	roomevents "lan-im-go/services/rooms/events"
	"lan-im-go/shared/http/middleware"
)

func main() {
	time.Local = time.UTC
	gin.SetMode(gin.ReleaseMode)

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "root:123456@tcp(127.0.0.1:3306)/lan_im?charset=utf8mb4&parseTime=True&loc=UTC"
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		pkg.Fatalf("[Room Service] MySQL 连接失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		pkg.Fatalf("[Room Service] 获取数据库连接池失败: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxIdleConns(50)
	sqlDB.SetMaxOpenConns(200)
	sqlDB.SetConnMaxLifetime(time.Hour)

	config.InitRedis()
	defer config.RedisClient.Close()

	service := application.NewService(
		repository.NewRoomRepoImpl(db),
		repository.NewRoomMemberRepoImpl(db),
		roomevents.NewRedisNotifier(config.RedisClient),
	)
	module := roomapi.NewModule(service)

	router := gin.New()
	router.Use(middleware.RequestID(), gin.Recovery())
	router.Use(cors.New(cors.Config{
		AllowAllOrigins: true, AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true, MaxAge: 12 * time.Hour,
	}))
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	authorized := router.Group("/api/v1", middleware.JWTAuth())
	module.RegisterRoutes(authorized)

	port := os.Getenv("ROOM_SERVER_PORT")
	if port == "" {
		port = "8082"
	}
	server := &http.Server{
		Addr: ":" + port, Handler: router, ReadTimeout: 5 * time.Second,
		ReadHeaderTimeout: 3 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 15 * time.Second,
	}
	pkg.Infof("[Room Service] 服务启动，监听端口 :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		pkg.Fatalf("[Room Service] 启动失败: %v", err)
	}
}
