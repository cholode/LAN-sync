package api

import (
	"lan-im-go/services/admin/application"
	"lan-im-go/services/messages/storage"
)

var (
	adminRuntime     admin.RuntimeController
	Storage          storage.Provider
	adminFileService *admin.FileService
)

// InitRuntimeController 注入 IM 主服务控制面客户端。
func InitRuntimeController(runtime admin.RuntimeController) {
	adminRuntime = runtime
}

// InitFileStorage 初始化管理端文件存储提供者。
func InitFileStorage() {
	Storage = storage.New()
}
