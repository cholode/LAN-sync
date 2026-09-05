package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/services/admin/application"
)

var adminRAGService *adminservice.RAGService

// InitAdminRAGService 初始化 RAG 服务。
func InitAdminRAGService(svc *adminservice.RAGService) {
	adminRAGService = svc
}

// AdminRAGQueries 查询 RAG 查询记录。
// 路由：GET /api/v1/admin/rag/queries
func AdminRAGQueries(c *gin.Context) {
	if adminRAGService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RAG 服务未初始化"})
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
	roomID, _ := strconv.ParseInt(c.Query("room_id"), 10, 64)

	items, total, err := adminRAGService.ListQueries(c.Request.Context(), page, pageSize, roomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 RAG 查询记录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
