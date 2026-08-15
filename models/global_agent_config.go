package models

import "time"

// GlobalAgentConfig \u5168\u5c40 Agent \u9ed8\u8ba4\u914d\u7f6e\uff0c\u8868\u5185\u53ea\u4fdd\u7559 ID=1 \u7684\u5355\u884c\u914d\u7f6e\u3002
type GlobalAgentConfig struct {
	ID                     int64     `gorm:"primaryKey" json:"id"`
	GlobalEnabled          bool      `gorm:"type:tinyint(1);default:1" json:"global_enabled"`
	DefaultModel           string    `gorm:"type:varchar(64);default:'deepseek-chat'" json:"default_model"`
	EmbeddingModel         string    `gorm:"type:varchar(64);default:'text-embedding-v3'" json:"embedding_model"`
	Temperature            float64   `gorm:"type:decimal(3,2);default:0.70" json:"temperature"`
	MaxTokens              int       `gorm:"type:int;default:4096" json:"max_tokens"`
	RAGTopK                int       `gorm:"type:int;default:5" json:"rag_top_k"`
	RAGSimilarityThreshold float64   `gorm:"type:decimal(3,2);default:0.70" json:"rag_similarity_threshold"`
	ChunkSize              int       `gorm:"type:int;default:800" json:"chunk_size"`
	ChunkOverlap           int       `gorm:"type:int;default:120" json:"chunk_overlap"`
	ModerationEnabled      bool      `gorm:"type:tinyint(1);default:1" json:"moderation_enabled"`
	ModerationModel        string    `gorm:"type:varchar(64);default:'deepseek-chat'" json:"moderation_model"`
	ModerationThreshold    float64   `gorm:"type:decimal(3,2);default:0.75" json:"moderation_threshold"`
	ToolCallingEnabled     bool      `gorm:"type:tinyint(1);default:1" json:"tool_calling_enabled"`
	AutoKickEnabled        bool      `gorm:"type:tinyint(1);default:0" json:"auto_kick_enabled"`
	AutoBanEnabled         bool      `gorm:"type:tinyint(1);default:0" json:"auto_ban_enabled"`
	SystemPrompt           string    `gorm:"type:longtext" json:"system_prompt"`
	ModerationPrompt       string    `gorm:"type:longtext" json:"moderation_prompt"`
	RAGPrompt              string    `gorm:"type:longtext" json:"rag_prompt"`
	ToolCallingPrompt      string    `gorm:"type:longtext" json:"tool_calling_prompt"`
	UpdatedAt              time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedAt              time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (GlobalAgentConfig) TableName() string {
	return "global_agent_configs"
}

// AgentConfigHistory \u4fdd\u5b58\u5168\u5c40 Agent \u914d\u7f6e\u7684\u7248\u672c\u53d8\u66f4\u8bb0\u5f55\uff0c\u7528\u4e8e\u56de\u6eda\u3002
type AgentConfigHistory struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigID      int64     `gorm:"type:bigint;index:idx_agent_config_history;not null" json:"config_id"`
	Version       int64     `gorm:"type:bigint;not null;default:1" json:"version"`
	BeforeData    string    `gorm:"type:longtext" json:"before_data"`
	AfterData     string    `gorm:"type:longtext" json:"after_data"`
	AdminUserID   int64     `gorm:"type:bigint;index:idx_agent_config_admin;not null;default:0" json:"admin_user_id"`
	AdminUsername string    `gorm:"type:varchar(64);default:''" json:"admin_username"`
	CreatedAt     time.Time `gorm:"index:idx_agent_config_created" json:"created_at"`
}

func (AgentConfigHistory) TableName() string {
	return "agent_config_histories"
}
