package api

import (
	//"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"lan-im-go/core"
	//"lan-im-go/models"
	"context"
	"lan-im-go/cache"
	"lan-im-go/internal/metrics"
	"lan-im-go/pkg"
	"lan-im-go/repository"
	"net/http"
	"sync"
	"time"
)

// WebSocket协议升级器
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// 跨域校验，生产环境需配置合法域名，防止CSRF攻击
	CheckOrigin: func(r *http.Request) bool {
		// 生产环境替换为正式域名校验
		// return strings.Contains(r.Header.Get("Origin"), "yourdomain.com")
		return true // 开发环境放行跨域
	},
	WriteBufferPool: &sync.Pool{New: func() interface{} { return make([]byte, 4096) }},
}

// WsEndpoint WebSocket连接入口
// 路由：authorized.GET("/ws", api.WsEndpoint(hub))
// 前端连接地址：ws://ip:port/api/v1/ws?token=JWT令牌

var CurrentNodeID = metrics.NodeID()

func WsEndpoint(hub *core.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 身份验证
		userID, exists := c.Get("user_id")
		if !exists {
			pkg.Infof("用户身份信息不存在\n")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "身份验证失败，连接拒绝"})
			return
		}
		realUserID := userID.(int64)

		// 2. 协议升级
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			pkg.Infof("[连接失败] WebSocket协议升级异常 UID:%d, Err:%v", realUserID, err)
			return
		}
		pkg.Infof("WebSocket连接建立成功 UID:%d\n", realUserID)
		connStart := time.Now()
		metrics.WSConnected()

		// 3. 极其核心的物理动作：全局宣告上线
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := cache.SetUserOnline(ctxTimeout, realUserID, CurrentNodeID); err != nil {
			pkg.Infof("[状态告警] UID:%d 写入全局 Redis 状态异常: %v", realUserID, err)
			// 注意：状态写入失败不应直接阻断连接，可做降级处理允许业务继续运行
		}
		cancel()

		// 4. 初始化群聊订阅
		roomIDs, err := repository.RoomMember.GetUserRoomIDs(realUserID)
		if err != nil {
			pkg.Infof("[连接警告] 获取用户%d群聊列表失败，使用空列表初始化", realUserID)
			roomIDs = []int64{}
		}

		// 5. 创建客户端实例
		client := &core.Client{
			Hub:    hub,
			UserID: realUserID,
			Conn:   conn,
			Send:   make(chan []byte, 512),
		}

		subscription := &core.Subscription{
			Client:  client,
			RoomIDs: roomIDs,
		}
		hub.Subscribe <- subscription

		// 6. 工业级防线：资源极致回收与状态宣告下线
		defer func() {
			metrics.WSDisconnected(time.Since(connStart), "normal")

			// A. 本地物理连接与路由表清退
			hub.Unsubscribe <- subscription
			conn.Close()

			// B. 全局分布式状态抹除
			ctxDel, cancelDel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := cache.SetUserOffline(ctxDel, realUserID); err != nil {
				pkg.Infof("[状态告警] UID:%d 删除全局 Redis 状态异常: %v", realUserID, err)
			}
			cancelDel()

			pkg.Infof("[WebSocket] 用户%d连接已释放，全局状态已离线", realUserID)
		}()

		// 7. 启动读写泵
		go client.WritePump()
		client.ReadPump()
	}
}
