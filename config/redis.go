package config

import (
	"context"
	"lan-im-go/pkg"
	"os"

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

	if err := RedisClient.Ping(context.Background()).Err(); err != nil {
		pkg.Fatalf("Redis 链路断开，启动失败：%v", err)
	}
	pkg.Infoln("Redis 准备就绪")
}
