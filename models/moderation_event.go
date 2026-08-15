package models

import "time"

// ModerationEvent 记录 AI 内容审核事件信息，用于管理后台 Dashboard 展示与人工复核。
type ModerationEvent struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`                                                  // 主键自增 ID，数据库唯一标识
	MessageID     int64      `gorm:"type:bigint;index:idx_moderation_message;not null;default:0" json:"message_id"`       // 被审核的消息 ID，建立索引便于按消息查询
	RoomID        int64      `gorm:"type:bigint;index:idx_moderation_room;not null;default:0" json:"room_id"`             // 消息所在的聊天室 ID，建立索引便于按房间统计
	UserID        int64      `gorm:"type:bigint;index:idx_moderation_user;not null;default:0" json:"user_id"`             // 发送消息的用户 ID，建立索引便于追踪用户行为
	Username      string     `gorm:"type:varchar(64);default:''" json:"username"`                                         // 发送消息的用户名（冗余存储，便于后台展示）
	RoomName      string     `gorm:"type:varchar(128);default:''" json:"room_name"`                                       // 聊天室名称（冗余存储，便于后台展示）
	OriginalMsg   string     `gorm:"type:text" json:"original_msg"`                                                       // 被审核的原始消息内容
	Category      string     `gorm:"type:varchar(32);index:idx_moderation_category;default:'other'" json:"category"`      // 审核分类（如涉政、色情、广告、辱骂等），建立索引便于分类统计
	RiskLevel     string     `gorm:"type:varchar(16);index:idx_moderation_risk;default:'low'" json:"risk_level"`          // 风险等级（如 low/medium/high），建立索引便于按风险级别筛选
	RiskScore     float64    `gorm:"type:double;default:0" json:"risk_score"`                                             // 风险评分（0-1 或 0-100 区间，根据模型输出）
	ModelName     string     `gorm:"type:varchar(64);default:''" json:"model_name"`                                       // 执行审核的 AI 模型名称
	ModelResult   string     `gorm:"type:varchar(32);default:''" json:"model_result"`                                     // 模型判断的最终结果（如 pass/block/review）
	ModelReason   string     `gorm:"type:text" json:"model_reason"`                                                       // 模型给出判断结果的理由说明
	ToolParams    string     `gorm:"type:text" json:"tool_params"`                                                        // 调用审核工具时传入的参数（JSON 字符串）
	ToolResult    string     `gorm:"type:text" json:"tool_result"`                                                        // 审核工具返回的原始结果（JSON 字符串）
	PenaltyStatus string     `gorm:"type:varchar(32);index:idx_moderation_penalty;default:'none'" json:"penalty_status"`  // 处罚状态（如 none/warning/muted/banned），建立索引便于查询处罚记录
	ReviewStatus  string     `gorm:"type:varchar(32);index:idx_moderation_review;default:'pending'" json:"review_status"` // 人工复核状态（如 pending/approved/rejected），建立索引便于待办处理
	ReviewedBy    int64      `gorm:"type:bigint;default:0" json:"reviewed_by"`                                            // 执行复核的管理员 ID（0 表示未复核）
	ReviewedAt    *time.Time `gorm:"type:datetime(3)" json:"reviewed_at"`                                                 // 复核完成时间（指针类型，可为空表示未复核）
	CreatedAt     time.Time  `gorm:"index:idx_moderation_created" json:"created_at"`                                      // 审核事件创建时间，建立索引便于按时间范围检索
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`                                                    // 记录最后更新时间，由数据库自动维护
}

// TableName 返回该结构体对应的数据库表名
func (ModerationEvent) TableName() string {
	return "moderation_events"
}
