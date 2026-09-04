// Package gateways owns transport routing and cross-cutting HTTP concerns.
// Business implementations live in their domain packages (messages, files, api).
package gateways

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"lan-im-go/pkg"
	adminservice "lan-im-go/services/admin/application"
	"lan-im-go/services/files/api"
	"lan-im-go/services/gateway/handlers"
	"lan-im-go/services/gateway/websocket"
	"lan-im-go/services/messages/api"
	"lan-im-go/shared/http/middleware"
	"lan-im-go/shared/observability/metrics"
)

// Dependencies are the runtime capabilities needed by the HTTP gateway.
// This explicit boundary makes the gateway independently replaceable later.
type Dependencies struct {
	Hub          *core.Hub
	DB           *gorm.DB
	Files        *files.Module
	Messages     *messages.Module
	ErrorService *adminservice.ErrorCenterService
	FrontendDir  string
}

// NewRouter constructs the public HTTP gateway without starting a listener.
func NewRouter(deps Dependencies) *gin.Engine {
	gin.SetMode(gin.DebugMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.RecoveryWithErrorRecorder(deps.ErrorService))
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		metrics.ObserveAPIRequest(c.Writer.Status(), time.Since(start))
	})
	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true, AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"}, ExposeHeaders: []string{"Content-Length"},
		AllowCredentials: true, MaxAge: 12 * time.Hour,
	}))

	public := r.Group("/api/v1")
	public.POST("/register", api.RegisterHandler)
	public.POST("/login", api.LoginHandler)
	deps.Files.RegisterPublicRoutes(public)

	authorized := r.Group("/api/v1")
	authorized.Use(middleware.JWTAuth())
	authorized.GET("/ws", api.WsEndpoint(deps.Hub))
	deps.Files.RegisterAuthorizedRoutes(authorized)
	deps.Messages.RegisterRoutes(authorized)
	registerRoomRoutes(authorized, deps.Hub)
	frontend := deps.FrontendDir
	if frontend == "" {
		frontend = "./frontend/dist"
	}
	r.Static("/assets", frontend+"/assets")
	r.GET("/", func(c *gin.Context) { c.File(frontend + "/index.html") })
	r.NoRoute(func(c *gin.Context) { c.File(frontend + "/index.html") })
	return r
}

func registerRoomRoutes(group *gin.RouterGroup, hub *core.Hub) {
	pkg.Infoln("进入WebSocket连接配置")
	group.POST("/rooms/:id/join", api.JoinRoom(hub))
	group.GET("/rooms/:id/members", api.GetRoomMembers())
	group.DELETE("/rooms/:id/members/:user_id", api.RemoveRoomMember(hub))
	group.DELETE("/rooms/:id/disband", api.OwnerDisbandRoom(hub))
	group.POST("/rooms", api.CreateRoom(hub))
	group.GET("/my_rooms", api.GetMyRooms())
}

// NewServer provides the gateway's independently testable/containerizable server.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{Addr: addr, Handler: handler, ReadTimeout: 5 * time.Second,
		ReadHeaderTimeout: 3 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 15 * time.Second}
}
