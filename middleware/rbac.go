package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"lan-im-go/models"
)

// RequireAdmin ?????????? super_admin?moderator?operator ???
// ?? JWTAuth ?????? user_role?
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := getUserRole(c)
		if !ok || !models.HasPermission(role, models.PermDashboardRead) {
			c.JSON(http.StatusForbidden, gin.H{"error": "????????????"})
			c.Abort()
			return
		}
		c.Set("admin_role", role)
		c.Next()
	}
}

// RequirePermission ????????????
// ???admin.PATCH("/users/:id/ban", middleware.RequirePermission(models.PermUserBan), handler)
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := getUserRole(c)
		if !ok || !models.HasPermission(role, permission) {
			c.JSON(http.StatusForbidden, gin.H{"error": "????????????"})
			c.Abort()
			return
		}
		c.Set("admin_role", role)
		c.Next()
	}
}

// RequireAnyPermission ?????????????????????????
func RequireAnyPermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := getUserRole(c)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "?????????"})
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
			c.JSON(http.StatusForbidden, gin.H{"error": "?????????"})
			c.Abort()
			return
		}
		c.Set("admin_role", role)
		c.Next()
	}
}

func getUserRole(c *gin.Context) (int8, bool) {
	value, exists := c.Get("user_role")
	if !exists {
		return 0, false
	}
	role, ok := value.(int8)
	return role, ok
}
