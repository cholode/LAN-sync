package api

import (
	"context"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gorm.io/gorm"
	usersmodel "lan-im-go/services/users/models"

	adminservice "lan-im-go/services/admin/application"
	"lan-im-go/services/messages/storage"
	"lan-im-go/shared/http/middleware"
)

// UserStore 是管理端登录和用户删除所需的数据访问能力。
type UserStore interface {
	GetByUsernameContext(context.Context, string) (*usersmodel.User, error)
	SoftDeleteUser(int64) error
}

// ModuleDependencies 显式提供用户仓库、基础设施和运行时控制器。
type ModuleDependencies struct {
	Users             UserStore
	DB                *gorm.DB
	MessageCollection *mongo.Collection
	MessageStore      string
	Storage           storage.Provider
	Runtime           adminservice.RuntimeController
}

// Module 是管理控制面的可部署边界。
type Module struct {
	Users        UserStore
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
	ragService := adminservice.NewRAGService(deps.DB)
	moderationService := adminservice.NewModerationService(deps.DB, auditService)
	fileService := adminservice.NewFileService(deps.DB, provider, auditService)

	InitAdminRAGService(ragService)
	InitAdminModerationService(moderationService)
	InitAdminUserService(adminservice.NewUserService(deps.DB, messageStats, deps.Runtime, auditService))
	InitAdminFileServiceVar(fileService)
	InitAdminAgentConfigService(adminservice.NewAgentConfigService(deps.DB, auditService))
	InitAdminToolCallService(adminservice.NewToolCallService(deps.DB))
	InitAdminAuditService(auditService)
	InitAdminRoomService(adminservice.NewRoomService(deps.DB, messageStats, deps.Runtime, auditService))

	return &Module{Users: deps.Users, ErrorService: errorService, FileService: fileService}
}

// RegisterRoutes 挂载管理 API，并保持认证、授权和限流边界完整。
func (m *Module) RegisterRoutes(router *gin.Engine) {
	// 登录接口属于 Admin Service 的公开入口，不能套用 JWT 中间件。
	router.POST("/api/v1/admin/login", m.AdminLogin)

	admin := router.Group("/api/v1/admin")
	admin.Use(middleware.JWTAuth(), middleware.RequireAdmin(), middleware.AdminRateLimit(10, 30))
	RegisterAdminRoutes(admin)
}
