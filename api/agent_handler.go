package api

import (
	"lan-im-go/agent"
	"lan-im-go/models"
	"lan-im-go/repository"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AgentHandler Agent 管理 API 处理器
type AgentHandler struct {
	agentMgr *agent.AgentManager
	db       *gorm.DB
}

// NewAgentHandler 创建 Agent API 处理器
func NewAgentHandler(agentMgr *agent.AgentManager, db *gorm.DB) *AgentHandler {
	return &AgentHandler{agentMgr: agentMgr, db: db}
}

// ============================================================================
// Agent 启停
// ============================================================================

// EnableAgent 启用群 Agent
// POST /api/v1/rooms/:id/agent/enable
func (h *AgentHandler) EnableAgent(c *gin.Context) {
	roomID, err := parseRoomIDParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的房间ID"})
		return
	}

	userID := c.GetInt64("user_id")
	if !h.isRoomAdminOrOwner(roomID, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅群主或管理员可操作"})
		return
	}

	if err := h.agentMgr.AddAgent(c.Request.Context(), roomID); err != nil {
		log.Printf("[AgentAPI] 启用 Agent room=%d 失败: %v", roomID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "启用 Agent 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Agent 已启用"})
}

// DisableAgent 暂停群 Agent（保留向量和分块数据，可重新启用）
// POST /api/v1/rooms/:id/agent/disable
func (h *AgentHandler) DisableAgent(c *gin.Context) {
	roomID, err := parseRoomIDParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的房间ID"})
		return
	}

	userID := c.GetInt64("user_id")
	if !h.isRoomAdminOrOwner(roomID, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅群主或管理员可操作"})
		return
	}

	if err := h.agentMgr.PauseAgent(c.Request.Context(), roomID); err != nil {
		log.Printf("[AgentAPI] 暂停 Agent room=%d 失败: %v", roomID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "暂停 Agent 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Agent 已暂停"})
}

// RemoveAgent 移除群 Agent 并清空全部相关数据（不可恢复）
// DELETE /api/v1/rooms/:id/agent
func (h *AgentHandler) RemoveAgent(c *gin.Context) {
	roomID, err := parseRoomIDParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的房间ID"})
		return
	}

	userID := c.GetInt64("user_id")
	if !h.isRoomAdminOrOwner(roomID, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅群主或管理员可操作"})
		return
	}

	if err := h.agentMgr.RemoveAgent(c.Request.Context(), roomID); err != nil {
		log.Printf("[AgentAPI] 移除 Agent room=%d 失败: %v", roomID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "移除 Agent 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Agent 已移除（含全部数据清理）"})
}

// ============================================================================
// Agent 配置
// ============================================================================

// GetAgentConfig 获取群 Agent 配置
// GET /api/v1/rooms/:id/agent/config
func (h *AgentHandler) GetAgentConfig(c *gin.Context) {
	roomID, err := parseRoomIDParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的房间ID"})
		return
	}

	var cfg models.AgentConfig
	if err := h.db.Where("room_id = ?", roomID).First(&cfg).Error; err != nil {
		cfg = *models.DefaultAgentConfig(roomID)
	}

	c.JSON(http.StatusOK, gin.H{"config": cfg})
}

// UpdateAgentConfig 更新群 Agent 配置
// PUT /api/v1/rooms/:id/agent/config
func (h *AgentHandler) UpdateAgentConfig(c *gin.Context) {
	roomID, err := parseRoomIDParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的房间ID"})
		return
	}

	userID := c.GetInt64("user_id")
	if !h.isRoomAdminOrOwner(roomID, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅群主或管理员可操作"})
		return
	}

	var req struct {
		SystemPrompt      string  `json:"system_prompt"`
		TriggerMode       int8    `json:"trigger_mode"`
		TriggerWords      string  `json:"trigger_words"`
		MaxHistory        int     `json:"max_history"`
		Temperature       float64 `json:"temperature"`
		ModelName         string  `json:"model_name"`
		RAGEnabled        *bool   `json:"rag_enabled"`
		TopK              int     `json:"top_k"`
		SimilarityThold   float64 `json:"similarity_thold"`
		RerankEnabled     *bool   `json:"rerank_enabled"`
		MaxChunkTokens    int     `json:"max_chunk_tokens"`
		TopicChunkMinMsgs int     `json:"topic_chunk_min_msgs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误"})
		return
	}

	var cfg models.AgentConfig
	if err := h.db.Where("room_id = ?", roomID).First(&cfg).Error; err != nil {
		cfg = *models.DefaultAgentConfig(roomID)
	}

	if req.SystemPrompt != "" {
		cfg.SystemPrompt = req.SystemPrompt
	}
	if req.TriggerMode > 0 {
		cfg.TriggerMode = req.TriggerMode
	}
	if req.TriggerWords != "" {
		cfg.TriggerWords = req.TriggerWords
	}
	if req.MaxHistory > 0 {
		cfg.MaxHistory = req.MaxHistory
	}
	if req.Temperature > 0 {
		cfg.Temperature = req.Temperature
	}
	if req.ModelName != "" {
		cfg.ModelName = req.ModelName
	}
	if req.RAGEnabled != nil {
		cfg.RAGEnabled = *req.RAGEnabled
	}
	if req.TopK > 0 {
		cfg.TopK = req.TopK
	}
	if req.SimilarityThold > 0 {
		cfg.SimilarityThold = req.SimilarityThold
	}
	if req.RerankEnabled != nil {
		cfg.RerankEnabled = *req.RerankEnabled
	}
	if req.MaxChunkTokens > 0 {
		cfg.MaxChunkTokens = req.MaxChunkTokens
	}
	if req.TopicChunkMinMsgs > 0 {
		cfg.TopicChunkMinMsgs = req.TopicChunkMinMsgs
	}

	if err := h.db.Save(&cfg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置已更新", "config": cfg})
}

// ============================================================================
// 辅助方法
// ============================================================================

func parseRoomIDParam(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}

func (h *AgentHandler) isRoomAdminOrOwner(roomID, userID int64) bool {
	role, ok, err := repository.RoomMember.GetMemberRole(roomID, userID)
	if err != nil || !ok {
		return false
	}
	return role >= 2
}

// RegisterAgentRoutes 注册 Agent 相关路由
func RegisterAgentRoutes(r *gin.RouterGroup, agentMgr *agent.AgentManager, db *gorm.DB) {
	handler := NewAgentHandler(agentMgr, db)

	agentGroup := r.Group("/rooms/:id/agent")
	{
		agentGroup.POST("/enable", handler.EnableAgent)
		agentGroup.POST("/disable", handler.DisableAgent)
		agentGroup.DELETE("", handler.RemoveAgent)
		agentGroup.GET("/config", handler.GetAgentConfig)
		agentGroup.PUT("/config", handler.UpdateAgentConfig)
	}
}