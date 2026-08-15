package admincontrol

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"lan-im-go/internal/admin"
	"lan-im-go/internal/metrics"
)

const controlTokenHeader = "X-Admin-Control-Token"

type runtimeBundle struct {
	Runtime metrics.RuntimeSnapshot      `json:"runtime"`
	Agent   metrics.AgentRuntimeSnapshot `json:"agent"`
}

// RegisterInternalRoutes 在主 IM 服务内注册管理端控制面接口。
func RegisterInternalRoutes(router *gin.RouterGroup, token string, controller admin.RuntimeController) {
	router.Use(func(c *gin.Context) {
		if token != "" && c.GetHeader(controlTokenHeader) != token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "管理端控制面令牌无效"})
			return
		}
		c.Next()
	})

	router.GET("/runtime", func(c *gin.Context) {
		runtimeSnap, agentSnap, err := controller.RuntimeSnapshots(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, runtimeBundle{Runtime: runtimeSnap, Agent: agentSnap})
	})
	router.GET("/connections", func(c *gin.Context) {
		items, err := controller.ListConnections(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, items)
	})
	router.GET("/hub-stats", func(c *gin.Context) {
		stats, err := controller.HubStats(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stats)
	})
	router.POST("/users/:id/kick", func(c *gin.Context) {
		userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法的用户 ID"})
			return
		}
		if err := controller.KickUser(c.Request.Context(), userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	router.POST("/connections/:id/close", func(c *gin.Context) {
		if err := controller.CloseConnection(c.Request.Context(), c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	router.POST("/rooms/:id/disband", func(c *gin.Context) {
		roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法的房间 ID"})
			return
		}
		if err := controller.DisbandRoom(c.Request.Context(), roomID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	router.POST("/rooms/:id/members/:user_id/remove", func(c *gin.Context) {
		roomID, roomErr := strconv.ParseInt(c.Param("id"), 10, 64)
		userID, userErr := strconv.ParseInt(c.Param("user_id"), 10, 64)
		if roomErr != nil || userErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法的房间或用户 ID"})
			return
		}
		if err := controller.RemoveRoomMember(c.Request.Context(), roomID, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	router.POST("/agents/:id/add", func(c *gin.Context) {
		roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法的房间 ID"})
			return
		}
		if err := controller.AddAgent(c.Request.Context(), roomID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	router.POST("/agents/:id/pause", func(c *gin.Context) {
		roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法的房间 ID"})
			return
		}
		if err := controller.PauseAgent(c.Request.Context(), roomID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}
