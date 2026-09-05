package events

import (
	"context"
	"encoding/json"

	"github.com/go-redis/redis/v8"

	"lan-im-go/pkg"
)

const Channel = "im:room:events"

const (
	MemberJoined  = "member_joined"
	MemberLeft    = "member_left"
	RoomDisbanded = "room_disbanded"
)

type Event struct {
	Type   string `json:"type"`
	RoomID int64  `json:"room_id"`
	UserID int64  `json:"user_id,omitempty"`
}

// RedisNotifier 将房间状态变化发布给所有 Gateway 实例。
type RedisNotifier struct {
	client *redis.Client
}

func NewRedisNotifier(client *redis.Client) *RedisNotifier {
	return &RedisNotifier{client: client}
}

func (n *RedisNotifier) JoinRoom(userID, roomID int64) {
	n.publish(Event{Type: MemberJoined, RoomID: roomID, UserID: userID})
}

func (n *RedisNotifier) LeaveRoom(userID, roomID int64) {
	n.publish(Event{Type: MemberLeft, RoomID: roomID, UserID: userID})
}

func (n *RedisNotifier) DisbandRoom(roomID int64) {
	n.publish(Event{Type: RoomDisbanded, RoomID: roomID})
}

func (n *RedisNotifier) publish(event Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		pkg.Errorf("[Room Service] 房间事件序列化失败: %v", err)
		return
	}
	if err := n.client.Publish(context.Background(), Channel, payload).Err(); err != nil {
		// 数据库已经提交，通知失败不应把成功操作伪装成事务失败；用户重连时会从数据库恢复订阅。
		pkg.Errorf("[Room Service] 房间事件发布失败: %v", err)
	}
}
