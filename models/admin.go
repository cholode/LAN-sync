package models

import "time"

// 用户角色定义，对应数据库 users.role 字段（tinyint 类型）
const (
	RoleUser       int8 = 0 // 普通用户
	RoleSuperAdmin int8 = 1 // 超级管理员
	RoleModerator  int8 = 2 // 内容审核员
	RoleOperator   int8 = 3 // 运营人员
)

// 权限常量定义，用于细粒度权限控制
const (
	PermDashboardRead    = "dashboard.read"    // 查看 Dashboard 仪表盘
	PermUserRead         = "user.read"         // 查看用户信息
	PermUserBan          = "user.ban"          // 封禁用户
	PermUserKick         = "user.kick"         // 踢出用户
	PermUserRoleUpdate   = "user.role.update"  // 修改用户角色
	PermUserDelete       = "user.delete"       // 删除用户
	PermRoomRead         = "room.read"         // 查看聊天室信息
	PermRoomFreeze       = "room.freeze"       // 冻结聊天室
	PermRoomDelete       = "room.delete"       // 删除聊天室
	PermMessageRead      = "message.read"      // 查看消息
	PermMessageDelete    = "message.delete"    // 删除消息
	PermModerationRead   = "moderation.read"   // 查看内容审核记录
	PermModerationReview = "moderation.review" // 执行内容审核复核
	PermAgentRead        = "agent.read"        // 查看 Agent 配置
	PermAgentConfig      = "agent.config"      // 修改 Agent 配置
	PermFileRead         = "file.read"         // 查看文件信息
	PermFileDelete       = "file.delete"       // 删除文件
	PermConnectionRead   = "connection.read"   // 查看连接信息
	PermConnectionClose  = "connection.close"  // 关闭连接
	PermAuditRead        = "audit.read"        // 查看审计日志
	PermSystemRead       = "system.read"       // 查看系统信息
)

// rolePermissionMap 定义各角色拥有的权限集合。
// 超级管理员拥有所有权限，无需在此定义。
var rolePermissionMap = map[int8]map[string]struct{}{
	// 内容审核员权限：拥有大部分管理功能，但不能修改 Agent 配置
	RoleModerator: {
		PermDashboardRead:    {}, // 查看仪表盘
		PermUserRead:         {}, // 查看用户
		PermUserBan:          {}, // 封禁用户
		PermUserKick:         {}, // 踢出用户
		PermRoomRead:         {}, // 查看聊天室
		PermRoomFreeze:       {}, // 冻结聊天室
		PermRoomDelete:       {}, // 删除聊天室
		PermMessageRead:      {}, // 查看消息
		PermMessageDelete:    {}, // 删除消息
		PermModerationRead:   {}, // 查看审核记录
		PermModerationReview: {}, // 执行审核复核
		PermAgentRead:        {}, // 查看 Agent 配置（只读）
		PermFileRead:         {}, // 查看文件
		PermConnectionRead:   {}, // 查看连接
	},
	// 运营人员权限：只读权限为主，无删除、封禁等操作权限
	RoleOperator: {
		PermDashboardRead:  {}, // 查看仪表盘
		PermUserRead:       {}, // 查看用户
		PermRoomRead:       {}, // 查看聊天室
		PermMessageRead:    {}, // 查看消息
		PermModerationRead: {}, // 查看审核记录
		PermAgentRead:      {}, // 查看 Agent 配置
		PermFileRead:       {}, // 查看文件
		PermConnectionRead: {}, // 查看连接
		PermAuditRead:      {}, // 查看审计日志
		PermSystemRead:     {}, // 查看系统信息
	},
}

// RoleName 返回角色对应的字符串名称，用于日志记录和展示。
func RoleName(role int8) string {
	switch role {
	case RoleSuperAdmin:
		return "super_admin"
	case RoleModerator:
		return "moderator"
	case RoleOperator:
		return "operator"
	default:
		return "user"
	}
}

// HasPermission 检查指定角色是否拥有某项权限。
// 超级管理员默认拥有所有权限。
func HasPermission(role int8, permission string) bool {
	if role == RoleSuperAdmin {
		return true
	}
	perms, ok := rolePermissionMap[role]
	if !ok {
		return false
	}
	_, ok = perms[permission]
	return ok
}

// AdminAuditLog 超级管理员操作审计日志，记录所有管理后台的敏感操作。
type AdminAuditLog struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`                                        // 主键自增 ID，审计日志唯一标识
	AdminUserID   int64     `gorm:"type:bigint;index:idx_admin_audit_admin;not null" json:"admin_user_id"`     // 执行操作的管理员用户 ID，建立索引便于按管理员查询
	AdminUsername string    `gorm:"type:varchar(64);not null" json:"admin_username"`                           // 执行操作的管理员用户名（冗余存储）
	Action        string    `gorm:"type:varchar(64);index:idx_admin_audit_action;not null" json:"action"`      // 操作类型（如 ban_user、delete_message 等），建立索引便于按操作筛选
	TargetType    string    `gorm:"type:varchar(32);index:idx_admin_audit_target;not null" json:"target_type"` // 操作目标类型（如 user/room/message 等），建立索引便于按目标类型查询
	TargetID      string    `gorm:"type:varchar(64);not null;default:''" json:"target_id"`                     // 操作目标 ID（如用户 ID、聊天室 ID 等）
	BeforeData    string    `gorm:"type:mediumtext" json:"before_data"`                                        // 操作前的数据快照（JSON 字符串）
	AfterData     string    `gorm:"type:mediumtext" json:"after_data"`                                         // 操作后的数据快照（JSON 字符串）
	RequestID     string    `gorm:"type:varchar(64);default:''" json:"request_id"`                             // 请求 ID，用于关联日志和链路追踪
	RemoteIP      string    `gorm:"type:varchar(64);default:''" json:"remote_ip"`                              // 操作者的远程 IP 地址
	UserAgent     string    `gorm:"type:varchar(255);default:''" json:"user_agent"`                            // 操作者的浏览器 User-Agent
	Result        string    `gorm:"type:varchar(16);default:'success'" json:"result"`                          // 操作结果（如 success/failure）
	ErrorMessage  string    `gorm:"type:text" json:"error_message"`                                            // 操作失败时的错误信息
	CreatedAt     time.Time `gorm:"index:idx_admin_audit_created" json:"created_at"`                           // 审计日志创建时间，建立索引便于按时间范围检索
}

// TableName 返回该结构体对应的数据库表名
func (AdminAuditLog) TableName() string {
	return "admin_audit_logs"
}
