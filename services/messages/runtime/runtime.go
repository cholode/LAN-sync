// Package runtime 负责独立消息进程的依赖初始化和退出清理。
package runtime

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"lan-im-go/config"
	"lan-im-go/infrastructure"
	"lan-im-go/repository"
	messages "lan-im-go/services/messages/api"
	"lan-im-go/services/messages/search"
	"lan-im-go/shared/observability/metrics"
)

type Dependencies struct {
	DB         *gorm.DB
	Messages   repository.MessageRepository
	Membership repository.RoomMemberRepository
}

// Open 复用已有消息数据；建表仍由现有主进程负责，避免多个服务并发迁移。
func Open(ctx context.Context) (*Dependencies, error) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("必须设置 DB_DSN")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	deps := &Dependencies{DB: db}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetConnMaxLifetime(time.Hour)
	metrics.RegisterMySQLPoolMetrics(sqlDB)
	metrics.RegisterGORMMetrics(db, "mysql")
	config.InitRedis()
	deps.Messages = messages.NewMySQLRepository(db)
	if os.Getenv("MESSAGE_STORE") == "mongo" {
		infrastructure.InitMongo()
		deps.Messages = messages.NewMongoRepository(infrastructure.MessageCollection)
	}
	deps.Membership = repository.NewRoomMemberRepoImpl(db)
	if err := search.Init(ctx); err != nil {
		deps.Close()
		return nil, err
	}
	return deps, nil
}

func (d *Dependencies) Close() {
	search.Close()
	if config.RedisClient != nil {
		_ = config.RedisClient.Close()
	}
	if os.Getenv("MESSAGE_STORE") == "mongo" {
		infrastructure.CloseMongo()
	}
	if sqlDB, err := d.DB.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// Ready 用于就绪检查；探测错误不会向客户端暴露连接信息。
func (d *Dependencies) Ready(ctx context.Context) error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}
	if os.Getenv("MESSAGE_STORE") == "mongo" {
		if err := infrastructure.MongoClient.Ping(ctx, readpref.Primary()); err != nil {
			return err
		}
	}
	return config.RedisClient.Ping(ctx).Err()
}

// Serve 在收到退出信号后停止接收请求，并等待已有请求结束。
func Serve(ctx context.Context, srv *http.Server) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		shutdownCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
		}
	}()
	err := srv.ListenAndServe()
	cancel()
	<-done
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
