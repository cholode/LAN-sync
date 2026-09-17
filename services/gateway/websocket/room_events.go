package core

import (
	"context"
	"encoding/json"

	"github.com/go-redis/redis/v8"

	"lan-im-go/config"
	roomevents "lan-im-go/services/rooms/events"
	"lan-im-go/shared/observability/logger"
)

// StartRoomEventListener 将独立 Room Service 的成员变更同步到本机 Hub。
func StartRoomEventListener(ctx context.Context, hub *Hub) {
	pubsub := config.RedisClient.Subscribe(ctx, roomevents.Channel)
	defer pubsub.Close()
	if _, err := pubsub.Receive(ctx); err != nil {
		logger.Errorf("[Gateway] 订阅房间事件失败: %v", err)
		return
	}

	channel := pubsub.Channel(redis.WithChannelSize(1024))
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-channel:
			if !ok {
				return
			}
			var event roomevents.Event
			if err := json.Unmarshal([]byte(message.Payload), &event); err != nil {
				logger.Errorf("[Gateway] 无法解析房间事件: %v", err)
				continue
			}
			switch event.Type {
			case roomevents.MemberJoined:
				hub.JoinRoom(event.UserID, event.RoomID)
			case roomevents.MemberLeft:
				hub.LeaveRoom(event.UserID, event.RoomID)
			case roomevents.RoomDisbanded:
				hub.DisbandRoom(event.RoomID)
			default:
				logger.Errorf("[Gateway] 忽略未知房间事件: %s", event.Type)
			}
		}
	}
}
