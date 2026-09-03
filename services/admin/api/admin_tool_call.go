package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/services/admin/application"
)

var adminToolCallService *adminservice.ToolCallService

// InitAdminToolCallService 初始化 Tool Call 服务。
func InitAdminToolCallService(svc *adminservice.ToolCallService) {
	adminToolCallService = svc
}

// AdminToolCallList 查询 Tool Calling 运行记录。\n// GET /api/v1/admin/tool-calls
func AdminToolCallList(c *gin.Context) {
	if adminToolCallService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "服务未初始化"})
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
	userID, _ := strconv.ParseInt(c.DefaultQuery("user_id", "0"), 10, 64)
	roomID, _ := strconv.ParseInt(c.DefaultQuery("room_id", "0"), 10, 64)
	var success *bool
	if raw := c.Query("success"); raw != "" {
		value := raw == "true" || raw == "1"
		success = &value
	}
	start, _ := time.Parse(time.RFC3339, c.Query("start"))
	end, _ := time.Parse(time.RFC3339, c.Query("end"))

	items, total, err := adminToolCallService.List(c.Request.Context(), adminservice.ToolCallListQuery{
		Page:     page,
		PageSize: pageSize,
		ToolName: c.Query("tool_name"),
		UserID:   userID,
		RoomID:   roomID,
		Success:  success,
		Start:    start,
		End:      end,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询 Tool Call 记录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}
