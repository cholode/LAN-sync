package core

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"lan-im-go/cache"
	"lan-im-go/config"
	"lan-im-go/shared/observability/metrics"
	"os"
	"sync"
	"sync/atomic"
	//messagesmodel "lan-im-go/services/messages/models"
	"lan-im-go/shared/observability/logger"
	"strconv"
	"time"
)

func heartbeatEnabled() bool {
	raw, ok := os.LookupEnv("WS_HEARTBEAT_ENABLED")
	if !ok || raw == "" {
		return true
	}
	enabled, err := strconv.ParseBool(raw)
	return err != nil || enabled
}

// CurrentGatewayNodeID 当前网关节点 ID，用于标记该连接所属的服务节点。
var CurrentGatewayNodeID = metrics.NodeID()

const (
	// WebSocket 配置参数
	writeWait             = 10 * time.Second    // 写入超时时间
	pongWait              = 30 * time.Second    // 客户端心跳响应超时时间
	pingPeriod            = (pongWait * 9) / 10 // 服务端心跳发送频率
	maxMessageSize        = 4096                // 限制单条消息最大长度，防止超大消息占用过多内存
	writeBatchWindow      = 20 * time.Millisecond
	writeBatchMaxMessages = 64
	writeBatchMaxBytes    = 64 * 1024
)

// Client 客户端连接实体
type Client struct {
	Hub    *Hub
	UserID int64
	Conn   *websocket.Conn
	Send   chan OutboundMessage

	usernameMu sync.RWMutex
	username   string

	ConnID        string
	RemoteIP      string
	UserAgent     string
	ClientVersion string
	ConnectedAt   time.Time

	mu         sync.RWMutex
	active     bool
	roomIDs    map[int64]struct{}
	sendMu     sync.RWMutex
	sendClosed bool

	lastReadAt  atomic.Int64
	lastWriteAt atomic.Int64
}

// OutboundMessage 保留消息进入 Gateway 的服务端时间，用于统计纯服务端链路延迟。
type OutboundMessage struct {
	Payload          []byte
	GatewayArrivedAt time.Time
}

// SetLastRead 记录客户端最近一次读取消息的时间。
func (c *Client) SetLastRead(t time.Time) {
	c.lastReadAt.Store(t.UnixMilli())
}

// SetLastWrite 记录客户端最近一次写入消息的时间。
func (c *Client) SetLastWrite(t time.Time) {
	c.lastWriteAt.Store(t.UnixMilli())
}

// LastReadAt 返回客户端最近一次读取消息的时间。
func (c *Client) LastReadAt() time.Time {
	if value := c.lastReadAt.Load(); value > 0 {
		return time.UnixMilli(value)
	}
	return c.ConnectedAt
}

// LastWriteAt 返回客户端最近一次写入消息的时间。
func (c *Client) LastWriteAt() time.Time {
	if value := c.lastWriteAt.Load(); value > 0 {
		return time.UnixMilli(value)
	}
	return c.ConnectedAt
}

// SetUsername 更新当前连接的用户名。
func (c *Client) SetUsername(name string) {
	c.usernameMu.Lock()
	c.username = name
	c.usernameMu.Unlock()
}

// Username 返回当前连接的用户名。
func (c *Client) Username() string {
	c.usernameMu.RLock()
	defer c.usernameMu.RUnlock()
	return c.username
}

func (c *Client) activate() {
	c.mu.Lock()
	c.active = true
	if c.roomIDs == nil {
		c.roomIDs = make(map[int64]struct{})
	}
	c.mu.Unlock()
}

func (c *Client) isActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active
}

func (c *Client) RoomIDs() []int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]int64, 0, len(c.roomIDs))
	for roomID := range c.roomIDs {
		out = append(out, roomID)
	}
	return out
}

func (c *Client) closeSend() {
	if c.Send == nil {
		return
	}
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if c.sendClosed {
		return
	}
	c.sendClosed = true
	close(c.Send)
}

// TrySend 非阻塞地向客户端发送队列投递消息，发送队列已关闭时返回 false。
func (c *Client) TrySend(payload []byte, gatewayArrivedAt time.Time) bool {
	c.sendMu.RLock()
	defer c.sendMu.RUnlock()
	if c.sendClosed {
		return false
	}
	select {
	case c.Send <- OutboundMessage{Payload: payload, GatewayArrivedAt: gatewayArrivedAt}:
		return true
	default:
		return false
	}
}

// ReadPump 持续读取客户端消息，解析后投递到 Kafka 全局消息流。
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	if heartbeatEnabled() {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		c.Conn.SetPongHandler(func(string) error {
			c.Conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})
	}

	for {
		messageType, message, err := c.Conn.ReadMessage()
		if err != nil {
			metrics.ObserveWSReadError(err)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Infof("[消息读取异常] 用户 %d 连接异常断开: %v", c.UserID, err)
			}
			break
		}
		gatewayArrivedAt := time.Now()
		metrics.ObserveWSReadMessage(messageType)
		c.SetLastRead(gatewayArrivedAt)

		// 1. 强制要求前端上报 ClientMsgID
		var payload struct {
			RoomID      int64  `json:"room_id"`
			Content     string `json:"content"`
			ClientMsgID string `json:"client_msg_id"` // 分布式架构必备的幂等性
		}

		if err := json.Unmarshal(message, &payload); err != nil {
			logger.Infof("[消息解析失败] 用户 %d 发送了非法格式: %v", c.UserID, err)
			continue
		}

		if payload.ClientMsgID == "" {
			logger.Infof("[非法调用] 用户 %d 缺失防重发凭证，已拒绝处理", c.UserID)
			continue
		}

		// 2. 剥离本地闭环，注入 Kafka 全局流
		// 此处调用之前封装好的极速生产者实例
		// 注意：RoomID 需要转换为 string 形式作为 Kafka 的路由 Key，以保证同群消息的物理顺序
		roomIDStr := strconv.FormatInt(payload.RoomID, 10)

		// 设定单次投递的极端超时时间，防止底层 I/O 拖垮协程
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

		// 将消息的物化投递任务完全甩给底层中间件
		err = config.KafkaProducer.HandleIncomingMessageAt(
			ctx,
			roomIDStr,
			int(c.UserID),
			payload.Content,
			payload.ClientMsgID,
			gatewayArrivedAt,
		)
		cancel()

		if err != nil {
			// 如果 Kafka 发生严重物理宕机，需要考虑降级策略或通知客户端发送失败
			logger.Infof("无法投递至 Kafka，消息丢弃: %v", err)
			// 可选：向当前客户端回复系统异常错误码
			continue
		}

	}
}

// ReadPump 读取消息：接收客户端消息，解析后发送至消息中心
// 每个客户端连接仅启动一个协程执行该方法
// func (c *Client) ReadPump() {
// 	// 连接关闭时释放资源
// 	defer func() {
// 		c.Hub.Unregister(c) // 由Hub注销客户端并清理连接
// 		c.Conn.Close()
// 	}()

// 	// 设置消息读取限制和心跳处理
// 	c.Conn.SetReadLimit(maxMessageSize)
// 	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
// 	c.Conn.SetPongHandler(func(string) error {
// 		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
// 		return nil
// 	})

// 	for {
// 		messageType, message, err := c.Conn.ReadMessage()
// 		logger.Infof("收到客户端消息：%s\n", message)
// 		if err != nil {
// 			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
// 				logger.Infof("[消息读取异常] 用户 %d 连接异常断开: %v", c.UserID, err)
// 			}
// 			break
// 		}

// 		// 解析客户端消息
// 		var payload struct {
// 			RoomID  int64  `json:"room_id"`
// 			Content string `json:"content"`
// 		}
// 		var msg messagesmodel.Message
// 		if err := json.Unmarshal(message, &payload); err != nil {
// 			logger.Infof("[消息解析失败] 用户 %d 发送了非法的 JSON 格式消息", c.UserID)
// 			continue
// 		}
// 		// 安全校验：用户ID从服务端获取，禁止客户端伪造身份
// 		msg.SenderID = c.UserID
// 		msg.Content = payload.Content
// 		msg.CreatedAt = time.Now()
// 		msg.Type = 1
// 		msg.RoomID = payload.RoomID

// 		// 发送至消息中心进行广播
// 		c.Hub.Broadcast <- &msg
// 	}
// }

// WritePump 发送消息：从消息中心接收数据并发送给客户端
// WebSocket写入操作非并发安全，仅允许单个协程执行
func (c *Client) WritePump() {
	var pingTicker *time.Ticker
	var pingC <-chan time.Time
	if heartbeatEnabled() {
		pingTicker = time.NewTicker(pingPeriod)
		pingC = pingTicker.C
	}
	flushTimer := time.NewTimer(time.Hour)
	if !flushTimer.Stop() {
		<-flushTimer.C
	}

	var flushC <-chan time.Time
	batch := make([]OutboundMessage, 0, writeBatchMaxMessages)
	batchBytes := 0

	stopFlushTimer := func() {
		if flushC == nil {
			return
		}
		if !flushTimer.Stop() {
			select {
			case <-flushTimer.C:
			default:
			}
		}
		flushC = nil
	}

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}

		c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
		w, err := c.Conn.NextWriter(websocket.TextMessage)
		if err != nil {
			return err
		}

		for i, message := range batch {
			if i > 0 {
				if _, err = w.Write([]byte{'\n'}); err != nil {
					_ = w.Close()
					return err
				}
			}
			if _, err = w.Write(message.Payload); err != nil {
				_ = w.Close()
				return err
			}
		}
		if err = w.Close(); err != nil {
			return err
		}

		leftGatewayAt := time.Now()
		metrics.ObserveWSWriteBatch(websocket.TextMessage, len(batch))
		transitMessages := 0
		transitSeconds := 0.0
		oldestSeconds := 0.0
		for _, message := range batch {
			if message.GatewayArrivedAt.IsZero() {
				continue
			}
			seconds := leftGatewayAt.Sub(message.GatewayArrivedAt).Seconds()
			if seconds < 0 {
				continue
			}
			transitMessages++
			transitSeconds += seconds
			if seconds > oldestSeconds {
				oldestSeconds = seconds
			}
		}
		metrics.ObserveGatewayFrameTransit(transitMessages, transitSeconds, oldestSeconds)
		c.SetLastWrite(leftGatewayAt)
		for i := range batch {
			batch[i] = OutboundMessage{}
		}
		batch = batch[:0]
		batchBytes = 0
		metrics.SetWSSendQueueBacklog(0)
		return nil
	}

	defer func() {
		if pingTicker != nil {
			pingTicker.Stop()
		}
		stopFlushTimer()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				stopFlushTimer()
				if err := flush(); err != nil {
					metrics.ObserveWSWriteError(err)
					return
				}
				c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			batch = append(batch, message)
			batchBytes += len(message.Payload)
			metrics.SetWSSendQueueBacklog(len(c.Send) + len(batch))
			if len(batch) == 1 {
				flushTimer.Reset(writeBatchWindow)
				flushC = flushTimer.C
			}
			if len(batch) >= writeBatchMaxMessages || batchBytes >= writeBatchMaxBytes {
				stopFlushTimer()
				if err := flush(); err != nil {
					metrics.ObserveWSWriteError(err)
					return
				}
			}

		case <-flushC:
			flushC = nil
			if err := flush(); err != nil {
				metrics.ObserveWSWriteError(err)
				return
			}

		case <-pingC:
			// 定时发送心跳包，维持连接存活
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				metrics.ObserveWSWriteError(err)
				return
			}

			//刷新redis中用户登录失效时间
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			_ = cache.RenewUserConnectionOnline(ctx, c.UserID, CurrentGatewayNodeID, c.ConnID)
			cancel()
		}
	}
}
