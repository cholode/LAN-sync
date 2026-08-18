package repository

import (
	"time"

	"gorm.io/gorm"
	"lan-im-go/models"
)

// ============================================================================
// 数据访问层接口定义
// 规范：业务层禁止直接操作gorm.DB，仅允许调用以下接口方法
// ============================================================================

// UserRepository 用户数据访问接口
type UserRepository interface {
	// 基础用户操作
	CreateUser(user *models.User) error
	GetByUsername(username string) (*models.User, error)
	GetByID(id int64) (*models.User, error)
	// 按ID软删除用户
	SoftDeleteUser(id int64) error
}

// RoomRepository 群组数据访问接口
type RoomRepository interface {
	// 创建群组并添加创建者，基于数据库事务保证一致性
	CreateRoomWithCreator(room *models.Room, creatorID int64) error
	GetRoomByID(roomID int64) (*models.Room, error)
	// 按ID软删除群组
	SoftDeleteRoom(roomID int64) error
	// 查询用户加入的所有群组，优化查询性能避免N+1问题
	GetJoinedRooms(userID int64) ([]*models.Room, error)
	// 根据名称精确查询群组
	GetRoomByExactName(exactName string) (*models.Room, error)
	// 查询用户加入的群组及当前用户在该群的角色，避免循环查询角色造成 N+1
	GetJoinedRoomsWithRole(userID int64) ([]JoinedRoom, error)
}

// JoinedRoom 表示用户加入的群组及其在该群中的角色。
type JoinedRoom struct {
	ID           int64
	Name         string
	AgentEnabled bool
	CreatorID    int64
	CreatedAt    time.Time
	MemberRole   int8
}

// RoomMemberRepository 群成员数据访问接口
type RoomMemberRepository interface {
	// 群成员管理
	AddMember(roomID, userID int64, role int8) error
	RemoveMember(roomID, userID int64) error
	// 查询用户加入的所有群组ID，用于WebSocket初始化
	GetUserRoomIDs(userID int64) ([]int64, error)
	// 校验用户是否为群成员，用于权限验证
	CheckIsMember(roomID, userID int64) (bool, error)
	// GetMemberRole 查询当前用户在群内的角色；ok=false 表示非成员或记录不存在
	GetMemberRole(roomID, userID int64) (role int8, ok bool, err error)
	// 查询群成员详细信息
	GetRoomMembers(roomID int64) ([]*models.User, error)
	// 查询群成员及其在群内的角色，避免逐成员查询角色造成 N+1
	GetRoomMembersWithRoles(roomID int64) ([]RoomMemberWithRole, error)
}

// RoomMemberWithRole 表示群成员资料及其在群内的角色。
type RoomMemberWithRole struct {
	ID         int64
	Username   string
	Avatar     string
	MemberRole int8
}

// MessageRepository 消息数据访问接口
type MessageRepository interface {
	// 异步保存消息
	SaveMessage(msg *models.Message) error

	SaveMessageBatch(msgs []*models.Message) error
	// 基于游标分页查询历史消息，避免深分页性能问题
	GetHistoryByCursor(roomID int64, cursorMsgID int64, limit int) ([]*models.Message, error)
	// GetMessagesByTimeRange 返回 created_at 在 [start, end) 范围内的消息，按升序排列。
	GetMessagesByTimeRange(roomID int64, start, end time.Time, limit int) ([]models.Message, error)
	// GetMessagesAfterID 返回 id 大于 sinceID 的消息，按升序排列。
	GetMessagesAfterID(roomID int64, sinceID int64, limit int) ([]models.Message, error)
	// CountMessagesAfterID 统计 id 大于 sinceID 的消息数量。
	CountMessagesAfterID(roomID int64, sinceID int64) (int64, error)
	// SearchMessages 按房间、正文和可选条件搜索消息。它既是 Elasticsearch
	// 不可用时的可靠回退，也覆盖尚未完成异步索引的历史消息。
	SearchMessages(params MessageSearchParams) ([]*models.Message, int64, error)
	// 批量软删除指定用户在群组内的消息
	SoftDeleteUserMessagesInRoom(roomID int64, userID int64) error
}

type MessageSearchParams struct {
	RoomID   int64
	Keyword  string
	SenderID int64
	Start    time.Time
	End      time.Time
	Offset   int
	Limit    int
}

// ============================================================================
// 全局数据访问接口实例
// 统一管理所有接口实现，避免业务层重复创建实例
// ============================================================================

var (
	User       UserRepository
	Room       RoomRepository
	RoomMember RoomMemberRepository
	Message    MessageRepository
)

// InitRepositories 初始化数据访问层
// 需在数据库连接初始化完成后调用，完成依赖注入
func InitRepositories(db *gorm.DB, messageRepo MessageRepository) {
	User = NewUserRepoImpl(db)
	Room = NewRoomRepoImpl(db)
	RoomMember = NewRoomMemberRepoImpl(db)
	Message = messageRepo
}
