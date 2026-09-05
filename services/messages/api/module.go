// Package messages 负责消息持久化和消息查询 HTTP 接口。
// 它有意只暴露精简的注册入口，以便在不移动业务实现的情况下拆成独立服务。
package messages

import (
	"github.com/gin-gonic/gin"
	"lan-im-go/repository"
)

// Module 包含消息 HTTP 查询所需的契约。
type Module struct {
	Repository repository.MessageRepository
	Membership repository.RoomMemberRepository
}

func NewModule(repo repository.MessageRepository, membership repository.RoomMemberRepository) *Module {
	return &Module{Repository: repo, Membership: membership}
}

// RegisterRoutes 在已认证的 API 路由组上挂载消息查询接口。
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/rooms/:id/messages", m.GetChatHistory())
	group.GET("/rooms/:id/messages/search", m.SearchMessages())
}
