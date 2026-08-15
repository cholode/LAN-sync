package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

var adminUserService *adminservice.UserService

// InitAdminUserService ?????????
func InitAdminUserService(svc *adminservice.UserService) {
	adminUserService = svc
}

// AdminUserList ???????
// GET /api/v1/admin/users
func AdminUserList(c *gin.Context) {
	if adminUserService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "???????????"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "????????"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// AdminUserDetail ???????
// GET /api/v1/admin/users/:id
func AdminUserDetail(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "???? ID"})
		return
	}
	if adminUserService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "???????????"})
		return
	}
	detail, err := adminUserService.GetUserDetail(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "?????"})
		return
	}
	c.JSON(http.StatusOK, detail)
}

// AdminUserAction ??????????
// POST /api/v1/admin/users/:id/action
func AdminUserAction(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "???? ID"})
		return
	}
	var req struct {
		Action string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "???????"})
		return
	}
	if adminUserService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "???????????"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "??????????"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "????"})
}
