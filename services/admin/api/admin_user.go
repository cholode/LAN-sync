package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/services/admin/application"
)

var adminUserService *adminservice.UserService

// InitAdminUserService 初始化用户管理服务。
func InitAdminUserService(svc *adminservice.UserService) {
	adminUserService = svc
}

// AdminUserList 分页查询用户。
// 路由：GET /api/v1/admin/users
func AdminUserList(c *gin.Context) {
	if adminUserService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "用户服务未初始化"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	role := int8(-1)
	if raw := c.Query("role"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			role = int8(v)
		}
	}
	status := int8(-1)
	if raw := c.Query("status"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			status = int8(v)
		}
	}
	start, _ := time.Parse(time.RFC3339, c.Query("start"))
	end, _ := time.Parse(time.RFC3339, c.Query("end"))

	items, total, err := adminUserService.ListUsers(c.Request.Context(), adminservice.UserListQuery{
		Page:     page,
		PageSize: pageSize,
		Keyword:  c.Query("keyword"),
		Role:     role,
		Status:   status,
		Start:    start,
		End:      end,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// AdminUserDetail 查询用户详情。
// 路由：GET /api/v1/admin/users/:id
func AdminUserDetail(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的用户 ID"})
		return
	}
	if adminUserService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "用户服务未初始化"})
		return
	}
	detail, err := adminUserService.GetUserDetail(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, detail)
}

// AdminUserAction 执行用户管理动作。
// 路由：POST /api/v1/admin/users/:id/action
func AdminUserAction(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的用户 ID"})
		return
	}
	var req struct {
		Action string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少操作参数"})
		return
	}
	if adminUserService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "用户服务未初始化"})
		return
	}
	permission := userActionPermission(req.Action)
	if permission == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的用户操作"})
		return
	}
	if !requireActionPermission(c, permission) {
		return
	}
	adminID := c.GetInt64("user_id")
	adminName := c.GetString("admin_username")
	if adminName == "" {
		adminName = strconv.FormatInt(adminID, 10)
	}
	err = adminUserService.ApplyAction(c.Request.Context(), userID, adminservice.UserAction{
		Action:      req.Action,
		AdminUserID: adminID,
		AdminName:   adminName,
		RequestID:   c.GetString("request_id"),
		RemoteIP:    c.ClientIP(),
		UserAgent:   c.Request.UserAgent(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "操作成功"})
}
