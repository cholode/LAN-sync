package auth

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

// IsAdminRole 判断角色是否可以进入治理后台。
func IsAdminRole(role int8) bool {
	return role == RoleSuperAdmin || role == RoleModerator || role == RoleOperator
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
