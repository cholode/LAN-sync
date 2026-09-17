package models

import "time"

// GlobalAgentConfig 全局 Agent 默认配置，表内只保留 ID=1 的单行配置。
type GlobalAgentConfig struct {
	ID                     int64     `gorm:"primaryKey" json:"id"`                                                // 主键 ID，固定为 1，表示全局唯一配置
	GlobalEnabled          bool      `gorm:"type:tinyint(1);default:1" json:"global_enabled"`                     // 全局 Agent 功能是否启用
	DefaultModel           string    `gorm:"type:varchar(64);default:'deepseek-chat'" json:"default_model"`       // 默认使用的 LLM 模型名称
	EmbeddingModel         string    `gorm:"type:varchar(64);default:'text-embedding-v3'" json:"embedding_model"` // 默认使用的 Embedding 模型名称
	Temperature            float64   `gorm:"type:decimal(3,2);default:0.70" json:"temperature"`                   // LLM 采样温度，控制生成随机性（0-1 之间）
	MaxTokens              int       `gorm:"type:int;default:4096" json:"max_tokens"`                             // LLM 单次生成的最大 token 数
	RAGTopK                int       `gorm:"type:int;default:5" json:"rag_top_k"`                                 // RAG 检索时返回的最相似文档数量
	RAGSimilarityThreshold float64   `gorm:"type:decimal(3,2);default:0.70" json:"rag_similarity_threshold"`      // RAG 检索的相似度阈值，低于此值的结果会被过滤
	ChunkSize              int       `gorm:"type:int;default:800" json:"chunk_size"`                              // 文本分块大小（字符数）
	ChunkOverlap           int       `gorm:"type:int;default:120" json:"chunk_overlap"`                           // 相邻文本块之间的重叠字符数
	ModerationEnabled      bool      `gorm:"type:tinyint(1);default:1" json:"moderation_enabled"`                 // 是否启用内容审核功能
	ModerationModel        string    `gorm:"type:varchar(64);default:'deepseek-chat'" json:"moderation_model"`    // 内容审核使用的模型名称
	ModerationThreshold    float64   `gorm:"type:decimal(3,2);default:0.75" json:"moderation_threshold"`          // 内容审核的风险阈值，超过此值触发处罚
	ToolCallingEnabled     bool      `gorm:"type:tinyint(1);default:1" json:"tool_calling_enabled"`               // 是否启用工具调用功能
	AutoKickEnabled        bool      `gorm:"type:tinyint(1);default:0" json:"auto_kick_enabled"`                  // 是否启用自动踢人功能（审核高风险时）
	AutoBanEnabled         bool      `gorm:"type:tinyint(1);default:0" json:"auto_ban_enabled"`                   // 是否启用自动封禁功能（审核高风险时）
	SystemPrompt           string    `gorm:"type:longtext" json:"system_prompt"`                                  // 系统提示词，定义 Agent 的角色和行为
	ModerationPrompt       string    `gorm:"type:longtext" json:"moderation_prompt"`                              // 内容审核使用的提示词
	RAGPrompt              string    `gorm:"type:longtext" json:"rag_prompt"`                                     // RAG 检索增强生成使用的提示词模板
	ToolCallingPrompt      string    `gorm:"type:longtext" json:"tool_calling_prompt"`                            // 工具调用时使用的提示词
	UpdatedAt              time.Time `gorm:"autoUpdateTime" json:"updated_at"`                                    // 配置最后更新时间，由数据库自动维护
	CreatedAt              time.Time `gorm:"autoCreateTime" json:"created_at"`                                    // 配置创建时间，由数据库自动维护
}

// TableName 返回该结构体对应的数据库表名
func (GlobalAgentConfig) TableName() string {
	return "global_agent_configs"
}

// AgentConfigHistory 保存全局 Agent 配置的版本变更记录，用于回滚。
type AgentConfigHistory struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`                                               // 主键自增 ID，历史记录唯一标识
	ConfigID      int64     `gorm:"type:bigint;index:idx_agent_config_history;not null" json:"config_id"`             // 关联的配置 ID（对应 global_agent_configs 表的 ID）
	Version       int64     `gorm:"type:bigint;not null;default:1" json:"version"`                                    // 配置版本号，递增
	BeforeData    string    `gorm:"type:longtext" json:"before_data"`                                                 // 变更前的完整配置数据（JSON 字符串）
	AfterData     string    `gorm:"type:longtext" json:"after_data"`                                                  // 变更后的完整配置数据（JSON 字符串）
	AdminUserID   int64     `gorm:"type:bigint;index:idx_agent_config_admin;not null;default:0" json:"admin_user_id"` // 执行变更的管理员用户 ID
	AdminUsername string    `gorm:"type:varchar(64);default:''" json:"admin_username"`                                // 执行变更的管理员用户名（冗余存储）
	CreatedAt     time.Time `gorm:"index:idx_agent_config_created" json:"created_at"`                                 // 变更记录创建时间，建立索引便于按时间检索
}

// TableName 返回该结构体对应的数据库表名
func (AgentConfigHistory) TableName() string {
	return "agent_config_histories"
}
