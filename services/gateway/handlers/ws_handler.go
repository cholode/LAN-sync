package api

import (
	"fmt"
	//"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"lan-im-go/services/gateway/websocket"
	//"lan-im-go/models"
	"context"
	"lan-im-go/cache"
	"lan-im-go/pkg"
	"lan-im-go/repository"
	"lan-im-go/shared/observability/metrics"
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
		userID, exists := c.Get("user_id")
		if !exists {
			pkg.Infof("user identity missing\n")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "identity invalid"})
			return
		}
		realUserID := userID.(int64)

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			pkg.Infof("[connection failed] WebSocket upgrade error UID:%d Err:%v", realUserID, err)
			return
		}
		pkg.Infof("WebSocket connected UID:%d\n", realUserID)
		connStart := time.Now()
		metrics.WSConnected()

		client := &core.Client{
			Hub:           hub,
			UserID:        realUserID,
			Conn:          conn,
			Send:          make(chan []byte, 512),
			ConnID:        fmt.Sprintf("%d-%d", realUserID, time.Now().UnixNano()),
			RemoteIP:      c.ClientIP(),
			UserAgent:     c.Request.UserAgent(),
			ClientVersion: c.GetHeader("X-Client-Version"),
			ConnectedAt:   time.Now(),
		}

		hub.Register(client, []int64{})

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := cache.SetUserOnline(ctx, realUserID, CurrentNodeID); err != nil {
				pkg.Infof("[online warn] UID:%d Redis online state failed: %v", realUserID, err)
			}
		}()

		go func() {
			roomIDs, err := repository.RoomMember.GetUserRoomIDs(realUserID)
			if err != nil {
				pkg.Infof("[connect warn] load rooms failed UID:%d: %v", realUserID, err)
				return
			}
			hub.UpdateRooms(client, roomIDs)
		}()

		go func() {
			if user, err := repository.User.GetByID(realUserID); err == nil && user != nil {
				client.SetUsername(user.Username)
			}
		}()

		defer func() {
			metrics.WSDisconnected(time.Since(connStart), "normal")
			hub.Unregister(client)
			conn.Close()

			ctxDel, cancelDel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancelDel()
			if err := cache.SetUserOffline(ctxDel, realUserID); err != nil {
				pkg.Infof("[offline warn] UID:%d Redis offline state failed: %v", realUserID, err)
			}
		}()

		go client.WritePump()
		client.ReadPump()
	}
}
