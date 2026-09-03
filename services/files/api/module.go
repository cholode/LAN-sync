// Package files owns file upload metadata and object-storage HTTP endpoints.
// Its route boundary is intentionally independent from the application router.
package files

import (
	"github.com/gin-gonic/gin"
	adminservice "lan-im-go/services/admin/application"
	"lan-im-go/services/files/api/storage"
)

// Module is the file service composition root. Keeping provider dependencies
// here avoids coupling the gateway to storage implementation details.
type Module struct {
	Storage    storage.Provider
	AdminFiles *adminservice.FileService
}

// NewModule initializes the file module and its current compatibility globals.
func NewModule(provider storage.Provider, adminFiles *adminservice.FileService) *Module {
	return &Module{Storage: provider, AdminFiles: adminFiles}
}

// RegisterPublicRoutes mounts endpoints that do not require authentication.
func (m *Module) RegisterPublicRoutes(group *gin.RouterGroup) {
	group.GET("/download/*filepath", m.DownloadFile)
}

// RegisterAuthorizedRoutes mounts endpoints protected by the gateway JWT middleware.
func (m *Module) RegisterAuthorizedRoutes(group *gin.RouterGroup) {
	group.POST("/files/presign", m.PreSignUploadHandler)
	group.POST("/files/complete", m.CompleteUploadHandler)
}
