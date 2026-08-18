package api

import (
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/qdrant/go-client/qdrant"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gorm.io/gorm"

	adminservice "lan-im-go/internal/admin"
	"lan-im-go/internal/storage"
	"lan-im-go/middleware"
)

// ModuleDependencies contains the infrastructure required by the admin HTTP
// module. Runtime can be a local controller in the monolith or a gRPC client
// when the admin process is split out again.
type ModuleDependencies struct {
	DB                *gorm.DB
	Redis             *redis.Client
	MessageCollection *mongo.Collection
	MessageStore      string
	Storage           storage.Provider
	Runtime           adminservice.RuntimeController
	QdrantClient      *qdrant.Client
}

// Module is the deployable boundary of the admin control plane.
type Module struct {
	ErrorService *adminservice.ErrorCenterService
	FileService  *adminservice.FileService
}

// NewModule wires all admin services without deciding whether they run in the
// main process or in a standalone admin process.
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

// RegisterRoutes mounts the admin API with its authentication, authorization,
// and rate-limit boundary intact.
func (m *Module) RegisterRoutes(router *gin.Engine) {
	admin := router.Group("/api/v1/admin")
	admin.Use(middleware.JWTAuth(), middleware.RequireAdmin(), middleware.AdminRateLimit(10, 30))
	RegisterAdminRoutes(admin)
}
