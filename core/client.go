package core

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"lan-im-go/cache"
	"lan-im-go/config"
	//"lan-im-go/models"
	"lan-im-go/pkg"
	"strconv"
	"time"
)

var CurrentGatewayNodeID = "node-1-192.168.1.100"

const (
	// WebSocket 配置参数
	writeWait      = 10 * time.Second    // 写入超时时间
	pongWait       = 30 * time.Second    // 客户端心跳响应超时时间
	pingPeriod     = (pongWait * 9) / 10 // 服务端心跳发送频率
	maxMessageSize = 4096                // 限制单条消息最大长度，防止超大消息占用过多内存
)

// Client 客户端连接实体
type Client struct {
	Hub    *Hub
	UserID int64
	Conn   *websocket.Conn
	// 消息发送缓冲通道，使用字节数组提升性能
	Send chan []byte
}

// Subscription 订阅信息
type Subscription struct {
	Client  *Client
	RoomIDs []int64 // 操作关联的群聊集合
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unsubscribe <- &Subscription{Client: c, RoomIDs: nil}
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				pkg.Infof("[消息读取异常] 用户 %d 连接异常断开: %v", c.UserID, err)
			}
			break
		}

		// 1. 强制要求前端上报 ClientMsgID
		var payload struct {
			RoomID      int64  `json:"room_id"`
			Content     string `json:"content"`
			ClientMsgID string `json:"client_msg_id"` // 分布式架构必备的幂等性
		}

		if err := json.Unmarshal(message, &payload); err != nil {
			pkg.Infof("[消息解析失败] 用户 %d 发送了非法格式: %v", c.UserID, err)
			continue
		}

		if payload.ClientMsgID == "" {
			pkg.Infof("[非法调用] 用户 %d 缺失防重发凭证，已拒绝处理", c.UserID)
			continue
		}

		// 2. 剥离本地闭环，注入 Kafka 全局流
		// 此处调用之前封装好的极速生产者实例
		// 注意：RoomID 需要转换为 string 形式作为 Kafka 的路由 Key，以保证同群消息的物理顺序
		roomIDStr := strconv.FormatInt(payload.RoomID, 10)

		// 设定单次投递的极端超时时间，防止底层 I/O 拖垮协程
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

		// 将消息的物化投递任务完全甩给底层中间件
		err = config.KafkaProducer.HandleIncomingMessage(
			ctx,
			roomIDStr,
			int(c.UserID),
			payload.Content,
			payload.ClientMsgID,
		)
		cancel()

		if err != nil {
			// 如果 Kafka 发生严重物理宕机，需要考虑降级策略或通知客户端发送失败
			pkg.Infof("无法投递至 Kafka，消息丢弃: %v", err)
			// 可选：向当前客户端回复系统异常错误码
			continue
		}

		// 架构级定调：
		// 此时绝对不调用 c.Hub.Broadcast。
		// 你的网关协程在这一步已经完成了它的历史使命，可以立刻进行下一次循环，去接住用户的高频连发。
	}
}

// ReadPump 读取消息：接收客户端消息，解析后发送至消息中心
// 每个客户端连接仅启动一个协程执行该方法
// func (c *Client) ReadPump() {
// 	// 连接关闭时释放资源
// 	defer func() {
// 		c.Hub.Unsubscribe <- &Subscription{Client: c, RoomIDs: nil} // 由Hub注销客户端并清理连接
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
// 		_, message, err := c.Conn.ReadMessage()
// 		pkg.Infof("收到客户端消息：%s\n", message)
// 		if err != nil {
// 			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
// 				pkg.Infof("[消息读取异常] 用户 %d 连接异常断开: %v", c.UserID, err)
// 			}
// 			break
// 		}

// 		// 解析客户端消息
// 		var payload struct {
// 			RoomID  int64  `json:"room_id"`
// 			Content string `json:"content"`
// 		}
// 		var msg models.Message
// 		if err := json.Unmarshal(message, &payload); err != nil {
// 			pkg.Infof("[消息解析失败] 用户 %d 发送了非法的 JSON 格式消息", c.UserID)
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
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// 消息通道已关闭，断开客户端连接
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 写入消息数据
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 批量写入优化：合并积压消息，减少系统IO调用
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			// 定时发送心跳包，维持连接存活
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

			//刷新redis中用户登录失效时间
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			_ = cache.RenewUserOnline(ctx, c.UserID)
			cancel()
		}
	}
}
