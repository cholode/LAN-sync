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
		pipelineStartedAt := time.Now()
		if value, exists := c.Get("ws_pipeline_started_at"); exists {
			if startedAt, ok := value.(time.Time); ok {
				pipelineStartedAt = startedAt
			}
		}
		readyRecorded := false
		defer func() {
			if !readyRecorded {
				metrics.ObserveWSConnectionStage(metrics.WSStageReadyTotal, pipelineStartedAt, "failed")
			}
		}()

		userID, exists := c.Get("user_id")
		if !exists {
			pkg.Infof("user identity missing\n")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "identity invalid"})
			return
		}
		realUserID := userID.(int64)
		membershipStartedAt := time.Now()
		roomIDs, err := repository.RoomMember.GetUserRoomIDs(realUserID)
		if err != nil {
			metrics.ObserveWSConnectionStage(metrics.WSStageMembership, membershipStartedAt, "failed")
			pkg.Infof("[connection failed] load rooms failed UID:%d Err:%v", realUserID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load joined rooms"})
			return
		}
		metrics.ObserveWSConnectionStage(metrics.WSStageMembership, membershipStartedAt, "success")

		upgradeStartedAt := time.Now()
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			metrics.ObserveWSConnectionStage(metrics.WSStageUpgrade, upgradeStartedAt, "failed")
			pkg.Infof("[connection failed] WebSocket upgrade error UID:%d Err:%v", realUserID, err)
			return
		}
		metrics.ObserveWSConnectionStage(metrics.WSStageUpgrade, upgradeStartedAt, "success")
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

		// 注册时一次性挂到该用户已经加入的全部房间，避免先注册空连接、
		// 再异步补房间期间漏掉群消息。
		hubRegisterStartedAt := time.Now()
		hub.Register(client, roomIDs)
		metrics.ObserveWSConnectionStage(metrics.WSStageHubRegister, hubRegisterStartedAt, "success")

		redisOnlineStartedAt := time.Now()
		ctxOnline, cancelOnline := context.WithTimeout(context.Background(), 2*time.Second)
		onlineResult := "success"
		if err := cache.SetUserConnectionOnline(ctxOnline, realUserID, CurrentNodeID, client.ConnID); err != nil {
			onlineResult = "failed"
			pkg.Infof("[online warn] UID:%d Redis online state failed: %v", realUserID, err)
		}
		cancelOnline()
		metrics.ObserveWSConnectionStage(metrics.WSStageRedisOnline, redisOnlineStartedAt, onlineResult)
		readyResult := "success"
		if onlineResult != "success" {
			readyResult = "degraded"
		}

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
			if err := cache.SetUserConnectionOffline(ctxDel, realUserID, CurrentNodeID, client.ConnID); err != nil {
				pkg.Infof("[offline warn] UID:%d Redis offline state failed: %v", realUserID, err)
			}
		}()

		go client.WritePump()
		metrics.ObserveWSConnectionStage(metrics.WSStageReadyTotal, pipelineStartedAt, readyResult)
		readyRecorded = true
		client.ReadPump()
	}
}
