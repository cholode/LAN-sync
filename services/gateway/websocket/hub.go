package core

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"

	"lan-im-go/config"
	"lan-im-go/contracts/events"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"lan-im-go/shared/concurrency/taskpool"
	"lan-im-go/shared/observability/metrics"
)

const (
	// 房间成员数达到该阈值后，使用协程池并发分发消息。
	defaultPoolDispatchThreshold = 100
	defaultFanoutBatchSize       = 200

	defaultHubShardCount = 64
)

// RoomAction 表示客户端在房间内的动作，例如加入、离开或解散。
type RoomAction struct {
	UserID int64
	RoomID int64
	Action string
}

// Subscription 保留类型兼容旧测试和连接订阅语义。
type Subscription struct {
	Client  *Client
	RoomIDs []int64
}

// Hub 局部内存路由引擎，按用户和房间两个维度分片。
type Hub struct {
	shards            []*hubShard
	fanoutPool        *taskpool.Pool
	fanoutThreshold   int
	fanoutBatchSize   int
	releaseFanoutOnce sync.Once
}

type hubShard struct {
	hub   *Hub
	id    int
	mu    sync.RWMutex
	users map[int64]*Client
	rooms map[int64]map[*Client]bool

	forward    chan *models.Message
	killClient chan *Client
}

// NewHub 创建分片式 Hub，分片数可通过 HUB_SHARD_COUNT 配置。
func NewHub() *Hub {
	shardCount := defaultHubShardCount
	if raw := os.Getenv("HUB_SHARD_COUNT"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			shardCount = parsed
		}
	}
	return NewHubWithShards(shardCount)
}

// NewHubWithShards 创建指定分片数的 Hub。
func NewHubWithShards(shardCount int) *Hub {
	if shardCount <= 0 {
		shardCount = defaultHubShardCount
	}
	workerCount := positiveEnvInt("HUB_FANOUT_WORKERS", runtime.GOMAXPROCS(0)*4)
	pool, err := taskpool.New(workerCount)
	if err != nil {
		panic("create gateway fan-out pool: " + err.Error())
	}
	hub := &Hub{
		shards:          make([]*hubShard, shardCount),
		fanoutPool:      pool,
		fanoutThreshold: positiveEnvInt("HUB_FANOUT_THRESHOLD", defaultPoolDispatchThreshold),
		fanoutBatchSize: positiveEnvInt("HUB_FANOUT_BATCH_SIZE", defaultFanoutBatchSize),
	}
	for i := 0; i < shardCount; i++ {
		hub.shards[i] = &hubShard{
			hub:        hub,
			id:         i,
			users:      make(map[int64]*Client),
			rooms:      make(map[int64]map[*Client]bool),
			forward:    make(chan *models.Message, 1024),
			killClient: make(chan *Client, 64),
		}
	}
	pkg.Infof(
		"[Gateway FanoutPool] ready workers=%d threshold=%d batch_size=%d",
		workerCount,
		hub.fanoutThreshold,
		hub.fanoutBatchSize,
	)
	return hub
}

func positiveEnvInt(key string, fallback int) int {
	if raw := os.Getenv(key); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

// FanoutPool 返回当前 Gateway 的扇出协程池，供监控模块采集指标。
func (h *Hub) FanoutPool() *taskpool.Pool {
	return h.fanoutPool
}

func (h *Hub) shardForUser(userID int64) *hubShard {
	return h.shards[int(uint64(userID)%uint64(len(h.shards)))]
}

func (h *Hub) shardForRoom(roomID int64) *hubShard {
	return h.shards[int(uint64(roomID)%uint64(len(h.shards)))]
}

// Run 启动所有分片循环，直到上下文取消。
func (h *Hub) Run(ctx context.Context) {
	defer h.releaseFanoutOnce.Do(h.fanoutPool.Release)
	var wg sync.WaitGroup
	for _, shard := range h.shards {
		wg.Add(1)
		go func(s *hubShard) {
			defer wg.Done()
			s.run(ctx)
		}(shard)
	}
	wg.Wait()
}

func (s *hubShard) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-s.forward:
			payload, err := json.Marshal(msg)
			if err != nil {
				pkg.Infof("[Local Hub 异常] 无法序列化消息 %v", err)
				continue
			}
			s.dispatchMessage(msg, payload)
		case client := <-s.killClient:
			s.hub.Unregister(client)
		}
	}
}

// Register 将客户端注册到用户分片，并订阅初始房间集合。
func (h *Hub) Register(c *Client, roomIDs []int64) {
	if c == nil {
		return
	}

	c.mu.Lock()
	c.active = true
	if c.roomIDs == nil {
		c.roomIDs = make(map[int64]struct{}, len(roomIDs))
	}
	for roomID := range c.roomIDs {
		delete(c.roomIDs, roomID)
	}
	for _, roomID := range roomIDs {
		c.roomIDs[roomID] = struct{}{}
	}

	userShard := h.shardForUser(c.UserID)
	userShard.mu.Lock()
	userShard.users[c.UserID] = c
	userShard.mu.Unlock()

	for _, roomID := range roomIDs {
		h.addClientToRoom(c, roomID)
	}
	c.mu.Unlock()

	h.updateMetrics()
}

// Unregister 从所有分片移除客户端，并安全关闭发送通道。
func (h *Hub) Unregister(c *Client) {
	if c == nil {
		return
	}

	c.mu.Lock()
	if !c.active {
		c.mu.Unlock()
		return
	}
	c.active = false

	userShard := h.shardForUser(c.UserID)
	userShard.mu.Lock()
	if current := userShard.users[c.UserID]; current == c {
		delete(userShard.users, c.UserID)
	}
	userShard.mu.Unlock()

	roomIDs := make([]int64, 0, len(c.roomIDs))
	for roomID := range c.roomIDs {
		roomIDs = append(roomIDs, roomID)
	}
	for _, roomID := range roomIDs {
		h.removeClientFromRoom(c, roomID)
	}
	c.roomIDs = nil
	c.mu.Unlock()

	c.closeSend()
	h.updateMetrics()
}

// UpdateRooms 异步刷新客户端订阅的房间集合。
func (h *Hub) UpdateRooms(c *Client, roomIDs []int64) {
	if c == nil {
		return
	}

	c.mu.Lock()
	if !c.active {
		c.mu.Unlock()
		return
	}

	newRooms := make(map[int64]struct{}, len(roomIDs))
	for _, roomID := range roomIDs {
		newRooms[roomID] = struct{}{}
	}

	for roomID := range c.roomIDs {
		if _, ok := newRooms[roomID]; !ok {
			h.removeClientFromRoom(c, roomID)
		}
	}
	for roomID := range newRooms {
		if _, ok := c.roomIDs[roomID]; !ok {
			h.addClientToRoom(c, roomID)
		}
	}

	c.roomIDs = newRooms
	c.mu.Unlock()
	h.updateMetrics()
}

// JoinRoom 将在线用户加入指定房间。
func (h *Hub) JoinRoom(userID, roomID int64) {
	c := h.clientByUser(userID)
	if c == nil {
		return
	}

	c.mu.Lock()
	if !c.active {
		c.mu.Unlock()
		return
	}
	if c.roomIDs == nil {
		c.roomIDs = make(map[int64]struct{})
	}
	if _, exists := c.roomIDs[roomID]; exists {
		c.mu.Unlock()
		return
	}
	c.roomIDs[roomID] = struct{}{}
	h.addClientToRoom(c, roomID)
	c.mu.Unlock()
	h.updateMetrics()
}

// LeaveRoom 将在线用户移出指定房间。
func (h *Hub) LeaveRoom(userID, roomID int64) {
	c := h.clientByUser(userID)
	if c == nil {
		return
	}

	c.mu.Lock()
	if !c.active {
		c.mu.Unlock()
		return
	}
	if _, exists := c.roomIDs[roomID]; !exists {
		c.mu.Unlock()
		return
	}
	delete(c.roomIDs, roomID)
	h.removeClientFromRoom(c, roomID)
	c.mu.Unlock()
	h.updateMetrics()
}

// DisbandRoom 删除房间，并清理该房间所有在线客户端的订阅状态。
func (h *Hub) DisbandRoom(roomID int64) {
	roomShard := h.shardForRoom(roomID)

	roomShard.mu.Lock()
	clients := make([]*Client, 0, len(roomShard.rooms[roomID]))
	for client := range roomShard.rooms[roomID] {
		clients = append(clients, client)
	}
	delete(roomShard.rooms, roomID)
	roomShard.mu.Unlock()

	for _, client := range clients {
		client.mu.Lock()
		delete(client.roomIDs, roomID)
		client.mu.Unlock()
	}
	h.updateMetrics()
}

// Publish 将跨节点消息投递到房间对应分片。
func (h *Hub) Publish(msg *models.Message) {
	if msg == nil {
		return
	}
	shard := h.shardForRoom(msg.RoomID)
	select {
	case shard.forward <- msg:
	default:
		metrics.ObserveHubQueueDrop(msg.RoomID, "shard_forward_full")
	}
}

// Kick 强制关闭指定用户的 WebSocket 连接。
func (h *Hub) Kick(userID int64) {
	if client := h.clientByUser(userID); client != nil && client.Conn != nil {
		client.Conn.Close()
	}
}

// CloseConnection 按连接 ID 关闭指定 WebSocket 连接。
func (h *Hub) CloseConnection(connectionID string) {
	for _, shard := range h.shards {
		shard.mu.RLock()
		var target *Client
		for _, client := range shard.users {
			if client.ConnID == connectionID {
				target = client
				break
			}
		}
		shard.mu.RUnlock()

		if target != nil && target.Conn != nil {
			target.Conn.Close()
			return
		}
	}
}

func (h *Hub) clientByUser(userID int64) *Client {
	shard := h.shardForUser(userID)
	shard.mu.RLock()
	client := shard.users[userID]
	shard.mu.RUnlock()
	return client
}

func (h *Hub) addClientToRoom(c *Client, roomID int64) {
	shard := h.shardForRoom(roomID)
	shard.mu.Lock()
	if shard.rooms[roomID] == nil {
		shard.rooms[roomID] = make(map[*Client]bool)
	}
	shard.rooms[roomID][c] = true
	shard.mu.Unlock()
}

func (h *Hub) removeClientFromRoom(c *Client, roomID int64) {
	shard := h.shardForRoom(roomID)
	shard.mu.Lock()
	if clients := shard.rooms[roomID]; clients != nil {
		delete(clients, c)
		if len(clients) == 0 {
			delete(shard.rooms, roomID)
		}
	}
	shard.mu.Unlock()
}

func (h *Hub) updateMetrics() {
	stats := h.Stats()
	metrics.SetHubClientCount(stats.ClientCount)
	metrics.SetHubRoomCount(stats.RoomCount)
}

// Connections 返回当前 Hub 中所有客户端连接快照。
type ConnectionSnapshot struct {
	UserID        int64     `json:"user_id"`
	Username      string    `json:"username"`
	ConnectionID  string    `json:"connection_id"`
	RemoteIP      string    `json:"remote_ip"`
	UserAgent     string    `json:"user_agent"`
	ClientVersion string    `json:"client_version"`
	ConnectedAt   time.Time `json:"connected_at"`
	LastReadAt    time.Time `json:"last_read_at"`
	LastWriteAt   time.Time `json:"last_write_at"`
	SendQueueLen  int       `json:"send_queue_len"`
	RoomIDs       []int64   `json:"room_ids"`
}

func (h *Hub) Connections() []ConnectionSnapshot {
	var clients []*Client
	for _, shard := range h.shards {
		shard.mu.RLock()
		for _, client := range shard.users {
			clients = append(clients, client)
		}
		shard.mu.RUnlock()
	}

	out := make([]ConnectionSnapshot, 0, len(clients))
	for _, client := range clients {
		out = append(out, ConnectionSnapshot{
			UserID:        client.UserID,
			Username:      client.Username(),
			ConnectionID:  client.ConnID,
			RemoteIP:      client.RemoteIP,
			UserAgent:     client.UserAgent,
			ClientVersion: client.ClientVersion,
			ConnectedAt:   client.ConnectedAt,
			LastReadAt:    client.LastReadAt(),
			LastWriteAt:   client.LastWriteAt(),
			SendQueueLen:  len(client.Send),
			RoomIDs:       client.RoomIDs(),
		})
	}
	return out
}

// HubStats 表示 Hub 的客户端数量和房间数量统计。
type HubStats struct {
	ClientCount int
	RoomCount   int
}

// Stats 返回当前 Hub 的客户端与房间统计信息。
func (h *Hub) Stats() HubStats {
	var stats HubStats
	for _, shard := range h.shards {
		shard.mu.RLock()
		stats.ClientCount += len(shard.users)
		stats.RoomCount += len(shard.rooms)
		shard.mu.RUnlock()
	}
	return stats
}

// StartGlobalListener 启动 Redis Pub/Sub 全局监听，将跨节点广播消息转发到本地 Hub。
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
				ClientMsgID: envelope.ClientMsgID,
				Type:        envelope.Type,
				Content:     envelope.Content,
				CreatedAt:   envelope.CreatedAt,
			}
			localHub.Publish(msg)
		}
	}
}

// dispatchMessage 将消息分发给房间内所有客户端。
func (s *hubShard) dispatchMessage(msg *models.Message, payload []byte) {
	s.mu.RLock()
	clients := make([]*Client, 0, len(s.rooms[msg.RoomID]))
	for client := range s.rooms[msg.RoomID] {
		clients = append(clients, client)
	}
	s.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	start := time.Now()
	defer func() {
		metrics.ObserveHubDispatchLatency(msg.RoomID, time.Since(start).Seconds())
	}()

	if len(clients) < s.hub.fanoutThreshold {
		dispatched := 0
		for _, client := range clients {
			if client.TrySend(payload) {
				dispatched++
			} else {
				metrics.ObserveHubQueueDrop(msg.RoomID, "client_send_full")
				metrics.RecordWSSlowClient()
				s.requestEvict(client)
			}
		}
		metrics.ObserveHubDispatch(msg.RoomID, dispatched)
		return
	}

	var dispatched atomic.Int64
	var batches sync.WaitGroup
	// 在保证每个任务不超过 fanoutBatchSize 的前提下均匀分批。
	// 按群成员上限 2000 和默认批次 200 计算，最多生成 10 个任务。
	taskCount := (len(clients) + s.hub.fanoutBatchSize - 1) / s.hub.fanoutBatchSize
	if taskCount < 2 {
		// 达到大群阈值后至少拆成两个任务，确保进入协程池后能真正并发扇出。
		taskCount = 2
	}
	balancedBatchSize := (len(clients) + taskCount - 1) / taskCount
	for start := 0; start < len(clients); start += balancedBatchSize {
		end := min(start+balancedBatchSize, len(clients))
		batch := clients[start:end]
		batches.Add(1)
		task := func() {
			defer batches.Done()
			for _, client := range batch {
				if client.TrySend(payload) {
					dispatched.Add(1)
					continue
				}
				metrics.ObserveHubQueueDrop(msg.RoomID, "client_send_full")
				metrics.RecordWSSlowClient()
				s.requestEvict(client)
			}
		}
		if err := s.hub.fanoutPool.Submit(task); err != nil {
			// 协程池关闭期间提交失败时在当前协程执行，避免静默丢失已接收消息。
			task()
		}
	}
	batches.Wait()
	metrics.ObserveHubDispatch(msg.RoomID, int(dispatched.Load()))
}

func (s *hubShard) requestEvict(client *Client) {
	select {
	case s.hub.shardForUser(client.UserID).killClient <- client:
	default:
	}
}
