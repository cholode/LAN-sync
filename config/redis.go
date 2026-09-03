package config

import (
	"context"

	"lan-im-go/internal/metrics"
	"lan-im-go/pkg"

	"github.com/go-redis/redis/v8"
)

var RedisClient *redis.Client

func InitRedis() {
	cfg := Messaging().Redis

	RedisClient = redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	RedisClient.AddHook(metrics.NewRedisHook())
	metrics.RegisterRedisPoolMetrics(RedisClient)

	if err := RedisClient.Ping(context.Background()).Err(); err != nil {
		metrics.SetRedisUp(false)
		pkg.Fatalf("Redis 链路断开，启动失败：%v", err)
	}
	metrics.SetRedisUp(true)
	pkg.Infoln("Redis 准备就绪")
}
