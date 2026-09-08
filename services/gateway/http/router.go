// Package gateways 负责传输层路由和横切 HTTP 能力。
// 业务实现在各自的领域包中，例如 messages、files 和 api。
package gateways

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	adminservice "lan-im-go/services/admin/application"
	"lan-im-go/services/gateway/handlers"
	"lan-im-go/services/gateway/websocket"
	"lan-im-go/shared/http/middleware"
)

// Dependencies 表示 HTTP 网关在运行时所依赖的能力。
// 通过显式定义这个边界，可以让网关以后能够被独立替换。
type Dependencies struct {
	Hub          *core.Hub
	DB           *gorm.DB
	ErrorService *adminservice.ErrorCenterService
	FrontendDir  string
}

// NewRouter 用于构建对外提供服务的 HTTP 网关，
// 但这里只负责创建路由，不会真正启动监听端口。
func NewRouter(deps Dependencies) *gin.Engine {
	gin.SetMode(gin.DebugMode)

	r := gin.New()

	r.Use(middleware.APIMetrics("gateway"))
	r.Use(middleware.RequestID())
	r.Use(middleware.RecoveryWithErrorRecorder(deps.ErrorService))

	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true,

		AllowMethods: []string{
			"GET",
			"POST",
			"PUT",
			"PATCH",
			"DELETE",
			"OPTIONS",
		},

		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Authorization",
		},

		ExposeHeaders: []string{
			"Content-Length",
		},

		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	public := r.Group("/api/v1")

	public.POST("/register", api.RegisterHandler)
	public.POST("/login", api.LoginHandler)

	authorized := r.Group("/api/v1")

	authorized.Use(middleware.JWTAuth())

	authorized.GET("/ws", api.WsEndpoint(deps.Hub))

	authorized.POST("/rooms/:id/agent/enable", api.RoomAgentHandler(deps.DB, "enable"))
	authorized.POST("/rooms/:id/agent/disable", api.RoomAgentHandler(deps.DB, "disable"))
	authorized.DELETE("/rooms/:id/agent", api.RoomAgentHandler(deps.DB, "remove", deps.Hub.LeaveRoom))

	frontend := deps.FrontendDir
	if frontend == "" {
		frontend = "./frontend/dist"
	}

	r.Static("/assets", frontend+"/assets")

	r.GET("/", func(c *gin.Context) {
		c.File(frontend + "/index.html")
	})
	r.GET("/health/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	r.NoRoute(func(c *gin.Context) {
		// 已迁出的业务接口不能回落成 HTML，避免调用方误判请求成功。
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在，请通过统一入口访问对应业务服务"})
			return
		}
		c.File(frontend + "/index.html")
	})

	return r
}

// NewServer 用于创建 HTTP 网关对应的 Server 实例。
// 这个 Server 可以被独立测试，也方便后续放入容器中部署。
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: handler,

		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       15 * time.Second,
	}
}
