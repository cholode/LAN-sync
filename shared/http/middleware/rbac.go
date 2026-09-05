package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"lan-im-go/models"
)

// RequireAdmin 校验当前用户是否为管理员角色，例如 super_admin、moderator 或 operator。
// 依赖 JWTAuth 中间件写入的 user_role。
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := getUserRole(c)
		if !ok || !models.IsAdminRole(role) {
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，无法访问管理后台"})
			c.Abort()
			return
		}
		c.Set("admin_role", role)
		c.Next()
	}
}

// RequirePermission 校验当前管理员是否拥有指定权限。
// 例如：admin.PATCH("/users/:id/ban", middleware.RequirePermission(models.PermUserBan), handler)
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := getUserRole(c)
		if !ok || !models.HasPermission(role, permission) {
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，无法执行该操作"})
			c.Abort()
			return
		}
		c.Set("admin_role", role)
		c.Next()
	}
}

// RequireAnyPermission 校验当前管理员是否拥有任意一个指定权限。
func RequireAnyPermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := getUserRole(c)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "缺少管理员身份信息"})
			c.Abort()
			return
		}
		allowed := false
		for _, permission := range permissions {
			if models.HasPermission(role, permission) {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，无法执行该操作"})
			c.Abort()
			return
		}
		c.Set("admin_role", role)
		c.Next()
	}
}

// getUserRole 从 Gin 上下文中读取当前管理员角色。
func getUserRole(c *gin.Context) (int8, bool) {
	value, exists := c.Get("user_role")
	if !exists {
		return 0, false
	}
	role, ok := value.(int8)
	return role, ok
}
