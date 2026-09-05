// Package files 负责文件上传元数据和对象存储 HTTP 接口。
// 它的路由边界有意与应用主路由保持独立。
package files

import (
	"github.com/gin-gonic/gin"
	adminservice "lan-im-go/services/admin/application"
	"lan-im-go/services/files/api/storage"
)

// Module 是文件服务的组装入口。在这里持有存储提供者依赖，
// 可以避免网关与存储实现细节耦合。
type Module struct {
	Storage    storage.Provider
	AdminFiles *adminservice.FileService
}

// NewModule 初始化文件模块及当前的兼容性全局变量。
func NewModule(provider storage.Provider, adminFiles *adminservice.FileService) *Module {
	return &Module{Storage: provider, AdminFiles: adminFiles}
}

// RegisterPublicRoutes 挂载无需认证的接口。
func (m *Module) RegisterPublicRoutes(group *gin.RouterGroup) {
	group.GET("/download/*filepath", m.DownloadFile)
}

// RegisterAuthorizedRoutes 挂载受网关 JWT 中间件保护的接口。
func (m *Module) RegisterAuthorizedRoutes(group *gin.RouterGroup) {
	group.POST("/files/presign", m.PreSignUploadHandler)
	group.POST("/files/complete", m.CompleteUploadHandler)
}
