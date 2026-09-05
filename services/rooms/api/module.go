// Package api 提供 Room Service 的 HTTP 接口。
package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"lan-im-go/services/rooms/application"
)

type Module struct {
	service *application.Service
}

func NewModule(service *application.Service) *Module {
	return &Module{service: service}
}

// RegisterRoutes 将房间领域接口挂载到已认证的路由组。
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/rooms", m.createRoom)
	group.GET("/rooms", m.searchRooms)
	group.GET("/my_rooms", m.joinedRooms)
	group.POST("/rooms/:id/join", m.joinRoom)
	group.GET("/rooms/:id/members", m.roomMembers)
	group.DELETE("/rooms/:id/members/:user_id", m.removeMember)
	group.DELETE("/rooms/:id/disband", m.disbandRoom)
}

func (m *Module) createRoom(c *gin.Context) {
	var request struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群聊名称不能为空"})
		return
	}
	room, err := m.service.CreateRoom(request.Name, c.GetInt64("user_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "创建群聊成功", "room_id": room.ID, "name": room.Name})
}

func (m *Module) searchRooms(c *gin.Context) {
	offset := queryInt(c, "offset", 0)
	limit := queryInt(c, "limit", 20)
	rooms, total, err := m.service.SearchRooms(c.Query("query"), offset, limit)
	if err != nil {
		writeError(c, err)
		return
	}
	items := make([]gin.H, 0, len(rooms))
	for _, room := range rooms {
		items = append(items, gin.H{
			"room_id": room.ID, "name": room.Name, "creator_id": room.CreatorID,
			"created_at": room.CreatedAt, "last_active_at": room.LastActiveAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"rooms": items, "total": total, "offset": max(offset, 0), "limit": min(max(limit, 1), 100)})
}

func (m *Module) joinedRooms(c *gin.Context) {
	rooms, err := m.service.JoinedRooms(c.GetInt64("user_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	items := make([]gin.H, 0, len(rooms))
	for _, room := range rooms {
		items = append(items, gin.H{
			"room_id": room.ID, "room_name": room.Name, "agent_enabled": room.AgentEnabled,
			"created_at": room.CreatedAt, "creator_id": room.CreatorID, "my_role": room.MemberRole,
		})
	}
	c.JSON(http.StatusOK, gin.H{"rooms": items})
}

func (m *Module) joinRoom(c *gin.Context) {
	roomID, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := m.service.JoinRoom(roomID, c.GetInt64("user_id")); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "加入群聊成功", "room_id": roomID})
}

func (m *Module) roomMembers(c *gin.Context) {
	roomID, ok := pathID(c, "id")
	if !ok {
		return
	}
	members, creatorID, err := m.service.RoomMembers(roomID, c.GetInt64("user_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	items := make([]gin.H, 0, len(members))
	for _, member := range members {
		items = append(items, gin.H{
			"user_id": member.ID, "username": member.Username, "avatar": member.Avatar,
			"role": member.MemberRole, "is_creator": member.ID == creatorID,
		})
	}
	c.JSON(http.StatusOK, gin.H{"room_id": roomID, "members": items})
}

func (m *Module) removeMember(c *gin.Context) {
	roomID, roomOK := pathID(c, "id")
	userID, userOK := pathID(c, "user_id")
	if !roomOK || !userOK {
		return
	}
	if err := m.service.RemoveMember(roomID, c.GetInt64("user_id"), userID, c.GetInt8("user_role")); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "操作成功"})
}

func (m *Module) disbandRoom(c *gin.Context) {
	roomID, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := m.service.DisbandRoom(roomID, c.GetInt64("user_id")); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "群聊已解散"})
}

func pathID(c *gin.Context, name string) (int64, bool) {
	value, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || value <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式非法"})
		return 0, false
	}
	return value, true
}

func queryInt(c *gin.Context, name string, fallback int) int {
	value, err := strconv.Atoi(c.Query(name))
	if err != nil {
		return fallback
	}
	return value
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, application.ErrInvalidRoomName):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, application.ErrRoomNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, application.ErrTargetNotMember):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, application.ErrNotRoomMember), errors.Is(err, application.ErrPermissionDenied), errors.Is(err, application.ErrCannotRemoveOwner):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "房间服务处理失败"})
	}
}
