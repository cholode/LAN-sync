package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"lan-im-go/models"
)

// AgentConfigService 管理全局 Agent 配置及其版本回滚。
type AgentConfigService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewAgentConfigService(db *gorm.DB, audit *AuditService) *AgentConfigService {
	return &AgentConfigService{db: db, audit: audit}
}

// GlobalAgentConfigInput 后台可修改的全局 Agent 配置字段。
type GlobalAgentConfigInput struct {
	GlobalEnabled          *bool   `json:"global_enabled"`
	DefaultModel           string  `json:"default_model"`
	EmbeddingModel         string  `json:"embedding_model"`
	Temperature            float64 `json:"temperature"`
	MaxTokens              int     `json:"max_tokens"`
	RAGTopK                int     `json:"rag_top_k"`
	RAGSimilarityThreshold float64 `json:"rag_similarity_threshold"`
	ChunkSize              int     `json:"chunk_size"`
	ChunkOverlap           int     `json:"chunk_overlap"`
	ModerationEnabled      *bool   `json:"moderation_enabled"`
	ModerationModel        string  `json:"moderation_model"`
	ModerationThreshold    float64 `json:"moderation_threshold"`
	ToolCallingEnabled     *bool   `json:"tool_calling_enabled"`
	AutoKickEnabled        *bool   `json:"auto_kick_enabled"`
	AutoBanEnabled         *bool   `json:"auto_ban_enabled"`
	SystemPrompt           string  `json:"system_prompt"`
	ModerationPrompt       string  `json:"moderation_prompt"`
	RAGPrompt              string  `json:"rag_prompt"`
	ToolCallingPrompt      string  `json:"tool_calling_prompt"`
}

func defaultGlobalAgentConfig() *models.GlobalAgentConfig {
	return &models.GlobalAgentConfig{
		ID:                     1,
		GlobalEnabled:          true,
		DefaultModel:           "deepseek-chat",
		EmbeddingModel:         "text-embedding-v3",
		Temperature:            0.70,
		MaxTokens:              4096,
		RAGTopK:                5,
		RAGSimilarityThreshold: 0.70,
		ChunkSize:              800,
		ChunkOverlap:           120,
		ModerationEnabled:      true,
		ModerationModel:        "deepseek-chat",
		ModerationThreshold:    0.75,
		ToolCallingEnabled:     true,
		AutoKickEnabled:        false,
		AutoBanEnabled:         false,
	}
}

func (s *AgentConfigService) getOrCreate(ctx context.Context) (*models.GlobalAgentConfig, error) {
	var cfg models.GlobalAgentConfig
	err := s.db.WithContext(ctx).First(&cfg, 1).Error
	if err == nil {
		return &cfg, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	defaults := defaultGlobalAgentConfig()
	if err := s.db.WithContext(ctx).Create(defaults).Error; err != nil {
		return nil, err
	}
	return defaults, nil
}

// Get 获取当前全局 Agent 配置。
func (s *AgentConfigService) Get(ctx context.Context) (*models.GlobalAgentConfig, error) {
	return s.getOrCreate(ctx)
}

// Update 更新全局 Agent 配置，并记录修改前后版本。
func (s *AgentConfigService) Update(ctx context.Context, input GlobalAgentConfigInput, action AuditAction) (*models.GlobalAgentConfig, error) {
	cfg, err := s.getOrCreate(ctx)
	if err != nil {
		return nil, err
	}
	before, _ := json.Marshal(cfg)

	if input.GlobalEnabled != nil {
		cfg.GlobalEnabled = *input.GlobalEnabled
	}
	if input.DefaultModel != "" {
		cfg.DefaultModel = input.DefaultModel
	}
	if input.EmbeddingModel != "" {
		cfg.EmbeddingModel = input.EmbeddingModel
	}
	if input.Temperature > 0 {
		cfg.Temperature = input.Temperature
	}
	if input.MaxTokens > 0 {
		cfg.MaxTokens = input.MaxTokens
	}
	if input.RAGTopK > 0 {
		cfg.RAGTopK = input.RAGTopK
	}
	if input.RAGSimilarityThreshold > 0 {
		cfg.RAGSimilarityThreshold = input.RAGSimilarityThreshold
	}
	if input.ChunkSize > 0 {
		cfg.ChunkSize = input.ChunkSize
	}
	if input.ChunkOverlap > 0 {
		cfg.ChunkOverlap = input.ChunkOverlap
	}
	if input.ModerationEnabled != nil {
		cfg.ModerationEnabled = *input.ModerationEnabled
	}
	if input.ModerationModel != "" {
		cfg.ModerationModel = input.ModerationModel
	}
	if input.ModerationThreshold > 0 {
		cfg.ModerationThreshold = input.ModerationThreshold
	}
	if input.ToolCallingEnabled != nil {
		cfg.ToolCallingEnabled = *input.ToolCallingEnabled
	}
	if input.AutoKickEnabled != nil {
		cfg.AutoKickEnabled = *input.AutoKickEnabled
	}
	if input.AutoBanEnabled != nil {
		cfg.AutoBanEnabled = *input.AutoBanEnabled
	}
	if input.SystemPrompt != "" {
		cfg.SystemPrompt = input.SystemPrompt
	}
	if input.ModerationPrompt != "" {
		cfg.ModerationPrompt = input.ModerationPrompt
	}
	if input.RAGPrompt != "" {
		cfg.RAGPrompt = input.RAGPrompt
	}
	if input.ToolCallingPrompt != "" {
		cfg.ToolCallingPrompt = input.ToolCallingPrompt
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(cfg).Error; err != nil {
			return err
		}
		after, _ := json.Marshal(cfg)
		return s.appendHistory(ctx, tx, cfg.ID, string(before), string(after), action)
	})
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, AuditEntry{
			AdminUserID:   action.AdminUserID,
			AdminUsername: action.AdminName,
			Action:        "agent.config.update",
			TargetType:    "global_agent_config",
			TargetID:      fmt.Sprintf("%d", cfg.ID),
			BeforeData:    string(before),
			AfterData:     string(afterJSON(cfg)),
			RequestID:     action.RequestID,
			RemoteIP:      action.RemoteIP,
			UserAgent:     action.UserAgent,
			Result:        "success",
		})
	}
	return cfg, nil
}

// HistoryItem 配置历史记录。
type HistoryItem struct {
	models.AgentConfigHistory
}

// History 分页查询配置历史。
func (s *AgentConfigService) History(ctx context.Context, page, pageSize int) ([]HistoryItem, int64, error) {
	var total int64
	query := s.db.WithContext(ctx).Model(&models.AgentConfigHistory{}).Where("config_id = ?", 1)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.AgentConfigHistory
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]HistoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, HistoryItem{AgentConfigHistory: row})
	}
	return items, total, nil
}

// Rollback 回滚到上一个版本，并写入新的历史记录。
func (s *AgentConfigService) Rollback(ctx context.Context, action AuditAction) (*models.GlobalAgentConfig, error) {
	var latest models.AgentConfigHistory
	if err := s.db.WithContext(ctx).Where("config_id = ?", 1).Order("id DESC").First(&latest).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("没有可回滚的历史版本")
		}
		return nil, err
	}
	if latest.BeforeData == "" {
		return nil, errors.New("历史版本缺少配置快照")
	}
	cfg, err := s.getOrCreate(ctx)
	if err != nil {
		return nil, err
	}
	before, _ := json.Marshal(cfg)
	if err := json.Unmarshal([]byte(latest.BeforeData), cfg); err != nil {
		return nil, fmt.Errorf("回滚配置失败: %w", err)
	}
	cfg.ID = 1
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(cfg).Error; err != nil {
			return err
		}
		after, _ := json.Marshal(cfg)
		return s.appendHistory(ctx, tx, cfg.ID, string(before), string(after), action)
	})
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, AuditEntry{
			AdminUserID:   action.AdminUserID,
			AdminUsername: action.AdminName,
			Action:        "agent.config.rollback",
			TargetType:    "global_agent_config",
			TargetID:      "1",
			RequestID:     action.RequestID,
			RemoteIP:      action.RemoteIP,
			UserAgent:     action.UserAgent,
			Result:        "success",
		})
	}
	return cfg, nil
}

func (s *AgentConfigService) appendHistory(ctx context.Context, tx *gorm.DB, configID int64, before, after string, action AuditAction) error {
	var maxVersion int64
	if err := tx.Model(&models.AgentConfigHistory{}).Where("config_id = ?", configID).Select("COALESCE(MAX(version), 0)").Scan(&maxVersion).Error; err != nil {
		return err
	}
	record := &models.AgentConfigHistory{
		ConfigID:      configID,
		Version:       maxVersion + 1,
		BeforeData:    before,
		AfterData:     after,
		AdminUserID:   action.AdminUserID,
		AdminUsername: action.AdminName,
		CreatedAt:     time.Now(),
	}
	return tx.Create(record).Error
}

func afterJSON(cfg *models.GlobalAgentConfig) string {
	raw, _ := json.Marshal(cfg)
	return string(raw)
}
