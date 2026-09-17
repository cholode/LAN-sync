package repository

import (
	roomsmodel "lan-im-go/services/rooms/models"
	usersmodel "lan-im-go/services/users/models"
	"time"
)

// RoomRepository 群组数据访问接口
type RoomRepository interface {
	// 创建群组并添加创建者，基于数据库事务保证一致性
	CreateRoomWithCreator(room *roomsmodel.Room, creatorID int64) error
	GetRoomByID(roomID int64) (*roomsmodel.Room, error)
	// 按ID软删除群组
	SoftDeleteRoom(roomID int64) error
	// 查询用户加入的所有群组，优化查询性能避免N+1问题
	GetJoinedRooms(userID int64) ([]*roomsmodel.Room, error)
	// 根据名称精确查询群组
	GetRoomByExactName(exactName string) (*roomsmodel.Room, error)
	// 查询用户加入的群组及当前用户在该群的角色，避免循环查询角色造成 N+1
	GetJoinedRoomsWithRole(userID int64) ([]JoinedRoom, error)
	// SearchRooms 按名称搜索可加入的普通群聊。
	SearchRooms(keyword string, offset, limit int) ([]*roomsmodel.Room, int64, error)
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
	GetRoomMembers(roomID int64) ([]*usersmodel.User, error)
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
