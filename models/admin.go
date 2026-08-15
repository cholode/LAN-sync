package models

import "time"

// ???????? users.role ?? tinyint ???
const (
	RoleUser       int8 = 0 // ????
	RoleSuperAdmin int8 = 1 // ?????
	RoleModerator  int8 = 2 // ?????
	RoleOperator   int8 = 3 // ?????
)

// ?????????????????????????????
const (
	PermDashboardRead    = "dashboard.read"
	PermUserRead         = "user.read"
	PermUserBan          = "user.ban"
	PermUserKick         = "user.kick"
	PermUserRoleUpdate   = "user.role.update"
	PermUserDelete       = "user.delete"
	PermRoomRead         = "room.read"
	PermRoomFreeze       = "room.freeze"
	PermRoomDelete       = "room.delete"
	PermMessageRead      = "message.read"
	PermMessageDelete    = "message.delete"
	PermModerationRead   = "moderation.read"
	PermModerationReview = "moderation.review"
	PermAgentRead        = "agent.read"
	PermAgentConfig      = "agent.config"
	PermFileRead         = "file.read"
	PermFileDelete       = "file.delete"
	PermConnectionRead   = "connection.read"
	PermConnectionClose  = "connection.close"
	PermAuditRead        = "audit.read"
	PermSystemRead       = "system.read"
)

// rolePermissionMap ???????????????????
// ???????????????
var rolePermissionMap = map[int8]map[string]struct{}{
	RoleModerator: {
		PermDashboardRead:    {},
		PermUserRead:         {},
		PermUserBan:          {},
		PermUserKick:         {},
		PermRoomRead:         {},
		PermRoomFreeze:       {},
		PermRoomDelete:       {},
		PermMessageRead:      {},
		PermMessageDelete:    {},
		PermModerationRead:   {},
		PermModerationReview: {},
		PermAgentRead:        {},
		PermFileRead:         {},
		PermConnectionRead:   {},
	},
	RoleOperator: {
		PermDashboardRead:  {},
		PermUserRead:       {},
		PermRoomRead:       {},
		PermMessageRead:    {},
		PermModerationRead: {},
		PermAgentRead:      {},
		PermFileRead:       {},
		PermConnectionRead: {},
		PermAuditRead:      {},
		PermSystemRead:     {},
	},
}

// RoleName ??????????????????????
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

// HasPermission ??????????????
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

// AdminAuditLog ????????????????????????
type AdminAuditLog struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	AdminUserID   int64     `gorm:"type:bigint;index:idx_admin_audit_admin;not null" json:"admin_user_id"`
	AdminUsername string    `gorm:"type:varchar(64);not null" json:"admin_username"`
	Action        string    `gorm:"type:varchar(64);index:idx_admin_audit_action;not null" json:"action"`
	TargetType    string    `gorm:"type:varchar(32);index:idx_admin_audit_target;not null" json:"target_type"`
	TargetID      string    `gorm:"type:varchar(64);not null;default:''" json:"target_id"`
	BeforeData    string    `gorm:"type:mediumtext" json:"before_data"`
	AfterData     string    `gorm:"type:mediumtext" json:"after_data"`
	RequestID     string    `gorm:"type:varchar(64);default:''" json:"request_id"`
	RemoteIP      string    `gorm:"type:varchar(64);default:''" json:"remote_ip"`
	UserAgent     string    `gorm:"type:varchar(255);default:''" json:"user_agent"`
	Result        string    `gorm:"type:varchar(16);default:'success'" json:"result"`
	ErrorMessage  string    `gorm:"type:text" json:"error_message"`
	CreatedAt     time.Time `gorm:"index:idx_admin_audit_created" json:"created_at"`
}

// TableName ?????????
func (AdminAuditLog) TableName() string {
	return "admin_audit_logs"
}
