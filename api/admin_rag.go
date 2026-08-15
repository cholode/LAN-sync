package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

var adminRAGService *adminservice.RAGService

// InitAdminRAGService ?? RAG ?????
func InitAdminRAGService(svc *adminservice.RAGService) {
	adminRAGService = svc
}

// AdminRAGDashboard ?? RAG/Qdrant ?????
// GET /api/v1/admin/dashboard/rag
func AdminRAGDashboard(c *gin.Context) {
	if adminRAGService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RAG ?????????"})
		return
	}
	data, err := adminRAGService.Dashboard(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "?? RAG ??????"})
		return
	}
	c.JSON(http.StatusOK, data)
}

// AdminRAGQueries ???? RAG ?????
// GET /api/v1/admin/rag/queries
func AdminRAGQueries(c *gin.Context) {
	if adminRAGService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RAG ?????????"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "?? RAG ??????"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
