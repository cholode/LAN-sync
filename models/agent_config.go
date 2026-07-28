package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// AgentConfig 群 Agent 配置模型
// 每个启用 Agent 的群拥有一条配置记录
type AgentConfig struct {
	ID     int64  `gorm:"primaryKey;autoIncrement"`
	RoomID int64  `gorm:"type:bigint;uniqueIndex;not null;comment:'所属群ID'"`

	// 基础配置
	SystemPrompt string  `gorm:"type:text;comment:'自定义系统提示词'"`
	TriggerMode  int8    `gorm:"type:tinyint;default:1;comment:'触发模式: 1=@提及 2=全部消息 3=关键词'"`
	TriggerWords string  `gorm:"type:varchar(512);comment:'触发关键词 JSON 数组'"`
	MaxHistory   int     `gorm:"type:int;default:20;comment:'上下文消息条数'"`
	Temperature  float64 `gorm:"type:decimal(3,2);default:0.70;comment:'LLM 温度参数'"`
	ModelName    string  `gorm:"type:varchar(64);default:'gpt-4o-mini';comment:'LLM 模型名'"`

	// RAG 配置
	RAGEnabled       bool    `gorm:"type:tinyint(1);default:1;comment:'是否启用 RAG'"`
	TopK             int     `gorm:"type:int;default:5;comment:'RAG 检索返回条数'"`
	SimilarityThold  float64 `gorm:"type:decimal(3,2);default:0.70;comment:'相似度阈值'"`
	RerankEnabled    bool    `gorm:"type:tinyint(1);default:1;comment:'是否启用重排序'"`
	MaxChunkTokens   int     `gorm:"type:int;default:4000;comment:'注入上下文的最大 Token'"`

	// 分块策略配置
	TopicChunkMinMsgs int    `gorm:"type:int;default:30;comment:'话题分块触发的最小消息数'"`
	TopicChunkModel   string `gorm:"type:varchar(64);default:'gpt-4o-mini';comment:'话题分块使用的 LLM 模型'"`

	CreatedAt time.Time             `gorm:"autoCreateTime"`
	UpdatedAt time.Time             `gorm:"autoUpdateTime"`
	DeletedAt soft_delete.DeletedAt `gorm:"type:bigint unsigned;index;softDelete:milli"`
}

func (AgentConfig) TableName() string {
	return "agent_configs"
}

// DefaultAgentConfig 返回默认配置
func DefaultAgentConfig(roomID int64) *AgentConfig {
	return &AgentConfig{
		RoomID:            roomID,
		SystemPrompt:      "",
		TriggerMode:       1,
		TriggerWords:      "[]",
		MaxHistory:        20,
		Temperature:       0.70,
		ModelName:         "gpt-4o-mini",
		RAGEnabled:        true,
		TopK:              5,
		SimilarityThold:   0.70,
		RerankEnabled:     true,
		MaxChunkTokens:    4000,
		TopicChunkMinMsgs: 30,
		TopicChunkModel:   "gpt-4o-mini",
	}
}