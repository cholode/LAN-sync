package config

import (
	"context"
	"os"

	"lan-im-go/internal/metrics"
	"lan-im-go/pkg"

	"github.com/go-redis/redis/v8"
)

var RedisClient *redis.Client

func InitRedis() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
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
