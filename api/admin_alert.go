package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

var adminAlertService *adminservice.AlertService

// InitAdminAlertService \u521d\u59cb\u5316\u544a\u8b66\u670d\u52a1\u3002
func InitAdminAlertService(svc *adminservice.AlertService) {
	adminAlertService = svc
}

// AdminAlertList \u67e5\u8be2\u544a\u8b66\u5217\u8868\u3002\n// GET /api/v1/admin/alerts
func AdminAlertList(c *gin.Context) {
	if adminAlertService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u544a\u8b66\u670d\u52a1\u672a\u521d\u59cb\u5316"})
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
	items, total, err := adminAlertService.List(c.Request.Context(), adminservice.AlertListQuery{
		Page:     page,
		PageSize: pageSize,
		Level:    c.Query("level"),
		Status:   c.Query("status"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u67e5\u8be2\u544a\u8b66\u5931\u8d25"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// AdminAlertEvaluate \u624b\u52a8\u89e6\u53d1\u544a\u8b66\u8bc4\u4f30\u3002\n// POST /api/v1/admin/alerts/evaluate
func AdminAlertEvaluate(c *gin.Context) {
	if adminAlertService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u544a\u8b66\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	items, err := adminAlertService.Evaluate(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u544a\u8b66\u8bc4\u4f30\u5931\u8d25"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// AdminAlertResolve \u6807\u8bb0\u544a\u8b66\u5df2\u5904\u7406\u3002\n// POST /api/v1/admin/alerts/:id/resolve
func AdminAlertResolve(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "\u975e\u6cd5\u544a\u8b66 ID"})
		return
	}
	if adminAlertService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u544a\u8b66\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	if err := adminAlertService.Resolve(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u5904\u7406\u544a\u8b66\u5931\u8d25"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "\u5df2\u6807\u8bb0\u4e3a\u5df2\u5904\u7406"})
}

// AdminAlertUnresolvedCount \u8fd4\u56de\u672a\u5904\u7406\u544a\u8b66\u6570\u91cf\u3002\n// GET /api/v1/admin/alerts/unresolved-count
func AdminAlertUnresolvedCount(c *gin.Context) {
	if adminAlertService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u544a\u8b66\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	count, err := adminAlertService.UnresolvedCount(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u7edf\u8ba1\u544a\u8b66\u5931\u8d25"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}
