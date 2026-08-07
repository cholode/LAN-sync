package core

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"lan-im-go/config"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"strconv"
)

// redisMessage 是 Redis Pub/Sub 消息的中间表示，JSON key 对齐生产者使用的 snake_case 格式
type redisMessage struct {
	RoomID      string `json:"room_id"`
	SenderID    int64  `json:"sender_id"`
	Content     string `json:"content"`
	ClientMsgID string `json:"client_msg_id"`
}

type RoomAction struct {
	UserID int64
	RoomID int64
	Action string
}

// Hub 局部内存路由引擎(Local Routing Engine)
// 物理定位：绝对无状态！重启不会丢失任何业务数据，只管理当前节点的 TCP 句柄。
type Hub struct {
	rooms map[int64]map[*Client]bool
	users map[int64]*Client

	Subscribe   chan *Subscription
	Unsubscribe chan *Subscription

	ForwardMessage chan *models.Message

	RoomActionChan chan *RoomAction
	Kick           chan int64
}

func NewHub() *Hub {
	return &Hub{
		rooms:       make(map[int64]map[*Client]bool),
		users:       make(map[int64]*Client),
		Subscribe:   make(chan *Subscription),
		Unsubscribe: make(chan *Subscription),

		ForwardMessage: make(chan *models.Message, 1024),

		RoomActionChan: make(chan *RoomAction, 100),
		Kick:           make(chan int64),
	}
}

func StartGlobalListener(ctx context.Context, localHub *Hub) {
	pubsub := config.RedisClient.PSubscribe(ctx, "im:broadcast:room:*")
	defer pubsub.Close()

	_, err := pubsub.Receive(ctx)
	if err != nil {
		pkg.Fatalf("[物理阻断] Redis Pub/Sub 全局总线连接失败: %v", err)
	}

	pkg.Infoln("[全局中枢] Redis 跨节点广播总线本地监听实例已成功点火...")

	//ch := pubsub.Channel()
	ch := pubsub.Channel(redis.WithChannelSize(10000))
	for {
		select {
		case <-ctx.Done():
			pkg.Infoln("[全局中枢] 收到系统关闭信号，Redis 监听协程安全退出")
			return
		case redisMsg := <-ch:
			// 1. 先反序列化到中间结构体（snake_case JSON → Go struct）
			var raw redisMessage
			if err := json.Unmarshal([]byte(redisMsg.Payload), &raw); err != nil {
				pkg.Infof("[数据脏污] 跨节点广播载荷解析失败 %v", err)
				continue
			}

			// 2. room_id 是字符串，转换回来
			roomID, err := strconv.ParseInt(raw.RoomID, 10, 64)
			if err != nil {
				pkg.Infof("[数据脏污] room_id 格式非法: %q %v", raw.RoomID, err)
				continue
			}

			// 3. 映射为业务模型
			msg := &models.Message{
				RoomID:      roomID,
				SenderID:    raw.SenderID,
				Content:     raw.Content,
				ClientMsgID: raw.ClientMsgID,
				Type:        1,
			}

			select {
			case localHub.ForwardMessage <- msg:
			default:
				pkg.Infoln("[性能预警] 本地 Hub 转发队列满，物理抛弃当前广播")
			}
		}
	}
}

func (h *Hub) Run(ctx context.Context) {
	pkg.Infoln("[Local Hub] 本地内存路由引擎已启动，等待 Redis 指令...")
	for {
		select {
		case <-ctx.Done():
			pkg.Infoln("[Local Hub] 收到系统关闭信号，本地路由引擎停止调度")
			return

		case sub := <-h.Subscribe:
			h.users[sub.Client.UserID] = sub.Client
			for _, roomID := range sub.RoomIDs {
				if h.rooms[roomID] == nil {
					h.rooms[roomID] = make(map[*Client]bool)
				}
				h.rooms[roomID][sub.Client] = true
			}

		case unsub := <-h.Unsubscribe:
			// 清理所有房间中的该客户端（包括 RoomIDs=nil 的情况）
			for rid, roomClients := range h.rooms {
				delete(roomClients, unsub.Client)
				if len(roomClients) == 0 {
					delete(h.rooms, rid)
				}
			}
			if _, ok := h.users[unsub.Client.UserID]; ok {
				delete(h.users, unsub.Client.UserID)
				close(unsub.Client.Send)
			}

		case msg := <-h.ForwardMessage:
			payload, err := json.Marshal(msg)
			if err != nil {
				pkg.Infof("[Local Hub 异常] 无法序列化消息 %v", err)
				continue
			}

			if clients, ok := h.rooms[msg.RoomID]; ok {
				for client := range clients {
					select {
					case client.Send <- payload:
					default:
						pkg.Infof("[Local Hub 绞杀] 客户端 %d 阻塞，物理断开", client.UserID)
						// 先摘除所有房间引用，再关通道，杜绝 send on closed channel
						for rid, roomClients := range h.rooms {
							delete(roomClients, client)
							if len(roomClients) == 0 {
								delete(h.rooms, rid)
							}
						}
						delete(h.users, client.UserID)
						close(client.Send)
					}
				}
			}

		case action := <-h.RoomActionChan:
			switch action.Action {
			case "join":
				if client, ok := h.users[action.UserID]; ok {
					if h.rooms[action.RoomID] == nil {
						h.rooms[action.RoomID] = make(map[*Client]bool)
					}
					h.rooms[action.RoomID][client] = true
				}
			case "leave":
				if client, ok := h.users[action.UserID]; ok {
					if h.rooms[action.RoomID] != nil {
						delete(h.rooms[action.RoomID], client)
					}
				}
			case "disband":
				if h.rooms[action.RoomID] != nil {
					delete(h.rooms, action.RoomID)
				}
			}

		case targetUserID := <-h.Kick:
			if client, ok := h.users[targetUserID]; ok {
				client.Conn.Close()
			}
		}
	}
}
