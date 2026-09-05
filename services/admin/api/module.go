package api

import (
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/qdrant/go-client/qdrant"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gorm.io/gorm"

	adminservice "lan-im-go/services/admin/application"
	"lan-im-go/services/messages/storage"
	"lan-im-go/shared/http/middleware"
)

// ModuleDependencies 包含管理端 HTTP 模块所需的基础设施。
// Runtime 在单体模式下可以是本地控制器，再次拆分管理服务后也可以替换为 gRPC 客户端。
type ModuleDependencies struct {
	DB                *gorm.DB
	Redis             *redis.Client
	MessageCollection *mongo.Collection
	MessageStore      string
	Storage           storage.Provider
	Runtime           adminservice.RuntimeController
	QdrantClient      *qdrant.Client
}

// Module 是管理控制面的可部署边界。
type Module struct {
	ErrorService *adminservice.ErrorCenterService
	FileService  *adminservice.FileService
}

// NewModule 组装全部管理服务，但不限定它们运行在主进程还是独立管理进程中。
func NewModule(deps ModuleDependencies) *Module {
	provider := deps.Storage
	if provider == nil {
		provider = storage.New()
	}

	InitRuntimeController(deps.Runtime)
	Storage = provider

	auditService := adminservice.NewAuditService(deps.DB)
	errorService := adminservice.NewErrorCenterService(deps.DB)
	messageStats := adminservice.NewMessageStatsStore(deps.DB, deps.MessageCollection, deps.MessageStore)
	ragService := adminservice.NewRAGService(deps.DB, deps.QdrantClient)
	moderationService := adminservice.NewModerationService(deps.DB, auditService)
	healthService := adminservice.NewHealthService(deps.DB, deps.Redis, provider, deps.Runtime, deps.QdrantClient)
	dashboardService := adminservice.NewDashboardService(
		deps.DB,
		deps.MessageCollection,
		deps.MessageStore,
		deps.Runtime,
		moderationService,
		ragService,
		healthService,
	)
	fileService := adminservice.NewFileService(deps.DB, provider, auditService)

	InitAdminDashboardService(dashboardService)
	InitAdminRAGService(ragService)
	InitAdminModerationService(moderationService)
	InitAdminHealthService(healthService)
	InitAdminUserService(adminservice.NewUserService(deps.DB, messageStats, deps.Runtime, auditService))
	InitAdminConnectionService(adminservice.NewConnectionService(deps.Runtime, auditService))
	InitAdminFileServiceVar(fileService)
	InitAdminAgentConfigService(adminservice.NewAgentConfigService(deps.DB, auditService))
	InitAdminToolCallService(adminservice.NewToolCallService(deps.DB))
	InitAdminErrorService(errorService)
	InitAdminAuditService(auditService)
	InitAdminAlertService(adminservice.NewAlertService(deps.DB, healthService, deps.Runtime))
	InitAdminRoomService(adminservice.NewRoomService(deps.DB, messageStats, deps.Runtime, auditService))

	return &Module{ErrorService: errorService, FileService: fileService}
}

// RegisterRoutes 挂载管理 API，并保持认证、授权和限流边界完整。
func (m *Module) RegisterRoutes(router *gin.Engine) {
	// 登录接口属于 Admin Service 的公开入口，不能套用 JWT 中间件。
	router.POST("/api/v1/admin/login", AdminLogin)

	admin := router.Group("/api/v1/admin")
	admin.Use(middleware.JWTAuth(), middleware.RequireAdmin(), middleware.AdminRateLimit(10, 30))
	RegisterAdminRoutes(admin)
}
