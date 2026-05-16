package cache

import (
	"context"
	"fmt"
	"lan-im-go/config"
	//"strconv"
	"time"
)

const (
	onlineKeyPrefix = "im:user:online:"
	onlineTTL       = 60 * time.Second
)

// SetUserOnline 用户建立连接时写入 Redis
func SetUserOnline(ctx context.Context, userID int64, nodeID string) error {
	key := fmt.Sprintf("%s%d", onlineKeyPrefix, userID)
	return config.RedisClient.Set(ctx, key, nodeID, onlineTTL).Err()
}

// SetUserOffline 用户主动断开时从 Redis 抹除
func SetUserOffline(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("%s%d", onlineKeyPrefix, userID)
	return config.RedisClient.Del(ctx, key).Err()
}

// RenewUserOnline 心跳续期防线：仅重置 TTL，不修改 Value
func RenewUserOnline(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("%s%d", onlineKeyPrefix, userID)
	return config.RedisClient.Expire(ctx, key, onlineTTL).Err()
}

// CheckUserOnline 极速鉴权：判断用户是否在全局任意节点存活
func CheckUserOnline(ctx context.Context, userID int64) (bool, string, error) {
	key := fmt.Sprintf("%s%d", onlineKeyPrefix, userID)
	nodeID, err := config.RedisClient.Get(ctx, key).Result()
	if err != nil {
		return false, "", nil
	}
	return true, nodeID, nil
}
