package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"lan-im-go/config"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"time"
)

const (
	onlineKeyPrefix     = "im:user:online:"
	onlineTTL           = 60 * time.Second
	roomLatestKeyPrefix = "im:room:latest:"
	roomLatestTTL       = 30 * time.Minute
	roomLatestMax       = 100
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

// CheckUserOnline 鉴权：判断用户是否在全局任意节点存活
func CheckUserOnline(ctx context.Context, userID int64) (bool, string, error) {
	key := fmt.Sprintf("%s%d", onlineKeyPrefix, userID)
	nodeID, err := config.RedisClient.Get(ctx, key).Result()
	if err != nil {
		return false, "", nil
	}
	return true, nodeID, nil
}

// ============================================================================
// 房间热点消息缓存（最新 100 条）
// ============================================================================

// CachedMsg 缓存消息体，与 API 响应对齐
type CachedMsg struct {
	ID        int64     `json:"id,string"`
	RoomID    int64     `json:"room_id,string"`
	SenderID  int64     `json:"sender_id,string"`
	Type      int8      `json:"type"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// GetLatestMessages 从 Redis 读取房间最新的 limit 条消息
func GetLatestMessages(ctx context.Context, roomID int64, limit int) ([]CachedMsg, error) {
	key := fmt.Sprintf("%s%d", roomLatestKeyPrefix, roomID)
	vals, err := config.RedisClient.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil || len(vals) == 0 {
		return nil, err
	}

	out := make([]CachedMsg, 0, len(vals))
	for _, v := range vals {
		var m CachedMsg
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// BackfillRoomCache 从 MySQL 回填 Redis 缓存（首次命中 miss 时调用）
func BackfillRoomCache(ctx context.Context, msgs []*models.Message) {
	if len(msgs) == 0 {
		return
	}
	pipe := config.RedisClient.Pipeline()
	rooms := make(map[int64]struct{}, len(msgs))

	for _, m := range msgs {
		payload, err := json.Marshal(CachedMsg{
			ID: m.ID, RoomID: m.RoomID, SenderID: m.SenderID,
			Type: m.Type, Content: m.Content, CreatedAt: m.CreatedAt,
		})
		if err != nil {
			continue
		}
		key := fmt.Sprintf("%s%d", roomLatestKeyPrefix, m.RoomID)
		pipe.LPush(ctx, key, string(payload))
		pipe.LTrim(ctx, key, 0, roomLatestMax-1)
		rooms[m.RoomID] = struct{}{}
	}

	for roomID := range rooms {
		key := fmt.Sprintf("%s%d", roomLatestKeyPrefix, roomID)
		pipe.Expire(ctx, key, roomLatestTTL)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		pkg.Infof("[Cache] Redis 回填热点消息失败: %v", err)
	}
}
