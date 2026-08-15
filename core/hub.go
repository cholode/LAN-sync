package core

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/go-redis/redis/v8"

	"lan-im-go/config"
	"lan-im-go/internal/metrics"
	"lan-im-go/internal/protocol"
	"lan-im-go/internal/taskpool"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"time"
)

const (
	// poolDispatchThreshold 房间成员数超过此阈值时，使用协程池分发消息
	poolDispatchThreshold = 100
)

type RoomAction struct {
	UserID int64
	RoomID int64
	Action string
}

// Hub 局部内存路由引擎(Local Routing Engine)
// 物理定位：绝对无状态！重启不会丢失任何业务数据，只管理当前节点的 TCP 句柄。
type Hub struct {
	mu    sync.RWMutex
	rooms map[int64]map[*Client]bool
	users map[int64]*Client

	Subscribe   chan *Subscription
	Unsubscribe chan *Subscription

	ForwardMessage chan *models.Message

	RoomActionChan chan *RoomAction
	Kick           chan int64

	// killClient 接收来自协程池中检测到的慢客户端，由 Hub 主循环统一清理
	killClient chan *Client
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
		killClient:     make(chan *Client, 64),
	}
}

// HubStats ? Hub ????????????????????????? map?
type HubStats struct {
	ClientCount int
	RoomCount   int
}

// Stats ?????? Hub ????????
func (h *Hub) Stats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return HubStats{
		ClientCount: len(h.users),
		RoomCount:   len(h.rooms),
	}
}

func StartGlobalListener(ctx context.Context, localHub *Hub) {
	pubsub := config.RedisClient.PSubscribe(ctx, "im:broadcast:room:*")
	defer pubsub.Close()

	_, err := pubsub.Receive(ctx)
	metrics.ObserveRedisPubSub("im:broadcast:room:*", "subscribe", err)
	if err != nil {
		pkg.Fatalf("[物理阻断] Redis Pub/Sub 全局总线连接失败: %v", err)
	}

	pkg.Infoln("[全局中枢] Redis 跨节点广播总线本地监听实例已成功点火...")

	ch := pubsub.Channel(redis.WithChannelSize(10000))
	for {
		select {
		case <-ctx.Done():
			pkg.Infoln("[全局中枢] 收到系统关闭信号，Redis 监听协程安全退出")
			return
		case redisMsg := <-ch:
			envelope, err := protocol.Unmarshal([]byte(redisMsg.Payload))
			if err != nil {
				pkg.Infof("[data dirty] cross-node broadcast parse failed: %v", err)
				continue
			}

			msg := &models.Message{
				RoomID:      envelope.RoomID,
				SenderID:    envelope.SenderID,
				Content:     envelope.Content,
				ClientMsgID: envelope.ClientMsgID,
				Type:        envelope.Type,
				CreatedAt:   envelope.CreatedAt,
			}

			select {
			case localHub.ForwardMessage <- msg:
			default:
				metrics.ObserveHubQueueDrop(msg.RoomID, "forward_queue_full")
				pkg.Infoln("[性能预警] 本地 Hub 转发队列满，物理抛弃当前广播")
			}
		}
	}
}

// dispatchMessage 将消息分发给房间内所有客户端
// 小房间直接内联分发，大房间通过协程池并行分发
func (h *Hub) dispatchMessage(msg *models.Message, payload []byte) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.rooms[msg.RoomID]))
	for client := range h.rooms[msg.RoomID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	start := time.Now()
	defer metrics.ObserveHubDispatchLatency(msg.RoomID, time.Since(start).Seconds())

	// ????????????
	if len(clients) < poolDispatchThreshold {
		dispatched := 0
		for _, client := range clients {
			select {
			case client.Send <- payload:
				dispatched++
			default:
				metrics.ObserveHubQueueDrop(msg.RoomID, "client_send_full")
				metrics.RecordWSSlowClient()
				h.requestEvict(client)
			}
		}
		metrics.ObserveHubDispatch(msg.RoomID, dispatched)
		return
	}

	// ?????????????????? Hub ???
	for _, client := range clients {
		c := client
		taskpool.Go(func() {
			select {
			case c.Send <- payload:
				metrics.ObserveHubDispatch(msg.RoomID, 1)
			default:
				metrics.ObserveHubQueueDrop(msg.RoomID, "client_send_full")
				metrics.RecordWSSlowClient()
				h.requestEvict(c)
			}
		})
	}
}

// requestEvict ?? Hub ??????????????????????? map?
func (h *Hub) requestEvict(client *Client) {
	select {
	case h.killClient <- client:
	default:
		// kill ???????Hub ?????????????
	}
}
func (h *Hub) evictClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	pkg.Infof("[Local Hub ??] ??? %d ???????", client.UserID)
	for rid, roomClients := range h.rooms {
		delete(roomClients, client)
		if len(roomClients) == 0 {
			delete(h.rooms, rid)
		}
	}
	if _, ok := h.users[client.UserID]; ok {
		delete(h.users, client.UserID)
	}
	// ????????????????recover ?? panic
	func() {
		defer func() { recover() }()
		close(client.Send)
	}()
}
func (h *Hub) Run(ctx context.Context) {
	pkg.Infoln("[Local Hub] 本地内存路由引擎已启动，等待 Redis 指令...")
	for {
		select {
		case <-ctx.Done():
			pkg.Infoln("[Local Hub] 收到系统关闭信号，本地路由引擎停止调度")
			return

		case sub := <-h.Subscribe:
			h.mu.Lock()
			h.users[sub.Client.UserID] = sub.Client
			for _, roomID := range sub.RoomIDs {
				if h.rooms[roomID] == nil {
					h.rooms[roomID] = make(map[*Client]bool)
				}
				h.rooms[roomID][sub.Client] = true
			}
			clientCount := len(h.users)
			roomCount := len(h.rooms)
			h.mu.Unlock()
			metrics.SetHubClientCount(clientCount)
			metrics.SetHubRoomCount(roomCount)

		case unsub := <-h.Unsubscribe:
			h.mu.Lock()
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
			clientCount := len(h.users)
			roomCount := len(h.rooms)
			h.mu.Unlock()
			metrics.SetHubClientCount(clientCount)
			metrics.SetHubRoomCount(roomCount)

		case msg := <-h.ForwardMessage:
			payload, err := json.Marshal(msg)
			if err != nil {
				pkg.Infof("[Local Hub 异常] 无法序列化消息 %v", err)
				continue
			}
			h.dispatchMessage(msg, payload)

		case action := <-h.RoomActionChan:
			h.mu.Lock()
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
			clientCount := len(h.users)
			roomCount := len(h.rooms)
			h.mu.Unlock()
			metrics.SetHubClientCount(clientCount)
			metrics.SetHubRoomCount(roomCount)

		case targetUserID := <-h.Kick:
			h.mu.RLock()
			client, ok := h.users[targetUserID]
			h.mu.RUnlock()
			if ok {
				client.Conn.Close()
			}

		case client := <-h.killClient:
			h.evictClient(client)
			stats := h.Stats()
			metrics.SetHubClientCount(stats.ClientCount)
			metrics.SetHubRoomCount(stats.RoomCount)
		}
	}
}
