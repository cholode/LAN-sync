// Package messages owns message persistence and message-query HTTP endpoints.
// It deliberately exposes a small registration surface so the package can be
// lifted into a standalone service without moving its business implementation.
package messages

import (
	"github.com/gin-gonic/gin"
	"lan-im-go/repository"
)

// Module contains the contracts required by message HTTP queries.
type Module struct {
	Repository repository.MessageRepository
	Membership repository.RoomMemberRepository
}

func NewModule(repo repository.MessageRepository, membership repository.RoomMemberRepository) *Module {
	return &Module{Repository: repo, Membership: membership}
}

// RegisterRoutes mounts message-query endpoints on an authenticated API group.
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/rooms/:id/messages", m.GetChatHistory())
	group.GET("/rooms/:id/messages/search", m.SearchMessages())
}
