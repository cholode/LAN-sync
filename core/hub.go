package core

import (
	"encoding/json"
	"log"

	"context"
	"lan-im-go/config"
	"lan-im-go/models"
)

type RoomAction struct {
	UserID int64
	RoomID int64
	Action string
}

// Hub 局部内存路由引擎 (Local Routing Engine)
// 物理定位：绝对无状态！重启不会丢失任何业务数据，只管理当前节点的 TCP 句柄。
type Hub struct {
	// 核心双索引结构：只存本地存活的连接
	rooms map[int64]map[*Client]bool
	users map[int64]*Client

	Subscribe   chan *Subscription
	Unsubscribe chan *Subscription

	// 【新增核心边界】：专门接收来自 Redis Pub/Sub 的跨节点广播指令
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

		// 缓冲通道：抵御 Redis 瞬间派发的消息洪峰
		ForwardMessage: make(chan *models.Message, 1024),

		RoomActionChan: make(chan *RoomAction, 100),
		Kick:           make(chan int64),
	}
}

// StartGlobalListener 全局广播监听状态机
// 此时属于 core 包内部方法，接收本包的 *Hub 实例，实现单向依赖链：core -> config/models
func StartGlobalListener(ctx context.Context, localHub *Hub) {
	// 强行向 Redis 内存总线发起模式订阅
	pubsub := config.RedisClient.PSubscribe(ctx, "im:broadcast:room:*")
	defer pubsub.Close()

	_, err := pubsub.Receive(ctx)
	if err != nil {
		log.Fatalf("[物理阻断] Redis Pub/Sub 全局总线连接失败: %v", err)
	}

	log.Println("[全局中枢] Redis 跨节点广播总线本地监听实例已成功点火...")

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			log.Println("[全局中枢] 收到系统关闭信号，Redis 监听协程安全退出")
			return
		case redisMsg := <-ch:
			// 1. 拦截 Redis 字节流并反序列化为业务模型
			var msg models.Message
			if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
				log.Printf("[数据脏污] 跨节点广播载荷解析失败: %v", err)
				continue
			}

			// 2. 极其安全的非阻塞派发：打入本地局部 Hub
			select {
			case localHub.ForwardMessage <- &msg:
			default:
				log.Println("[性能预警] 本地 Hub 转发队列满，物理抛弃当前广播")
			}
		}
	}
}

func (h *Hub) Run(ctx context.Context) {
	log.Println("[Local Hub] 本地内存路由引擎已启动，等待 Redis 指令...")
	for {
		select {
		case <-ctx.Done():
			log.Println("[Local Hub] 收到系统关闭信号，本地路由引擎停止调度")
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
			for _, roomID := range unsub.RoomIDs {
				if clients, ok := h.rooms[roomID]; ok {
					delete(clients, unsub.Client)
					if len(clients) == 0 {
						delete(h.rooms, roomID)
					}
				}
			}
			if _, ok := h.users[unsub.Client.UserID]; ok {
				delete(h.users, unsub.Client.UserID)
				close(unsub.Client.Send) // 物理斩断通道
			}

		case msg := <-h.ForwardMessage:
			// 【物理职责限定】：Hub 只管派发，不管落盘，不管 Kafka，那些都在前面做完了。
			// 1. 统一序列化，压榨 CPU 性能
			payload, err := json.Marshal(msg)
			if err != nil {
				log.Printf("[Local Hub 异常] 无法序列化消息: %v", err)
				continue
			}

			// 2. 寻找本地在线的群成员
			if clients, ok := h.rooms[msg.RoomID]; ok {
				for client := range clients {
					select {
					case client.Send <- payload:
					default:
						// 发送通道阻塞，说明客户端网络极度卡顿，直接断开该连接
						log.Printf("[Local Hub 绞杀] 客户端 %d 阻塞，物理断开", client.UserID)
						close(client.Send)
						delete(clients, client)
						delete(h.users, client.UserID)
					}
				}
			}

		case action := <-h.RoomActionChan:
			// 内存状态同步
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
				// 给客户端发个强制下线指令，然后断开
				client.Conn.Close()
			}
		}
	}
}
