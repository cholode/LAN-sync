package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"lan-im-go/models"
)

// adminHasPermission 从当前请求上下文中获取角色并校验权限。
func adminHasPermission(c *gin.Context, permission string) bool {
	value, ok := c.Get("user_role")
	if !ok {
		return false
	}
	role, ok := value.(int8)
	if !ok {
		return false
	}
	return models.HasPermission(role, permission)
}

// requireActionPermission 在合并的 action 接口内根据具体操作做粒度更细的权限控制。
func requireActionPermission(c *gin.Context, permission string) bool {
	if adminHasPermission(c, permission) {
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，无法执行该操作"})
	return false
}

// userActionPermission 返回用户管理 action 对应的权限点。
func userActionPermission(action string) string {
	switch action {
	case "ban", "unban":
		return models.PermUserBan
	case "role_super_admin", "role_moderator", "role_operator", "role_user":
		return models.PermUserRoleUpdate
	default:
		return ""
	}
}

// roomActionPermission 返回群聊管理 action 对应的权限点。
func roomActionPermission(action string) string {
	switch action {
	case "freeze", "unfreeze", "remove_member", "set_admin", "transfer_owner":
		return models.PermRoomFreeze
	case "disband":
		return models.PermRoomDelete
	case "agent_enable", "agent_disable":
		return models.PermAgentConfig
	case "moderation_enable", "moderation_disable":
		return models.PermModerationReview
	default:
		return ""
	}
}
