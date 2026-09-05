package repository

import (
	"strings"

	"gorm.io/gorm"
	"lan-im-go/models"
)

type roomRepository interface {
	// 底层封装事务，保证操作原子性
	CreateRoomWithCreator(room *models.Room, creatorID int64) error
	GetRoomByID(roomID int64) (*models.Room, error)
	SoftDeleteRoom(roomID int64) error
	GetJoinedRooms(userID int64) ([]*models.Room, error)
	GetRoomByExactName(exactName string) (*models.Room, error)
	GetJoinedRoomsWithRole(userID int64) ([]JoinedRoom, error)
	SearchRooms(keyword string, offset, limit int) ([]*models.Room, int64, error)
}

// SearchRooms 按名称搜索未解散的普通群聊，并返回分页总数。
func (r *roomRepoImpl) SearchRooms(keyword string, offset, limit int) ([]*models.Room, int64, error) {
	query := r.db.Model(&models.Room{}).Where("type = ? AND status = ?", 2, 0)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rooms []*models.Room
	if err := query.Order("last_active_at DESC, id DESC").Offset(offset).Limit(limit).Find(&rooms).Error; err != nil {
		return nil, 0, err
	}
	return rooms, total, nil
}

type roomRepoImpl struct {
	db *gorm.DB
}

func NewRoomRepoImpl(db *gorm.DB) roomRepository {
	return &roomRepoImpl{db: db}
}

// CreateRoomWithCreator 创建群聊并添加创建者，通过事务保证原子性
func (r *roomRepoImpl) CreateRoomWithCreator(room *models.Room, creatorID int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. 创建群聊数据
		if err := tx.Create(room).Error; err != nil {
			return err
		}
		// 2. 添加创建者为群成员并设置管理员权限 (Role: 2)
		member := &models.RoomMember{
			RoomID: room.ID,
			UserID: creatorID,
			Role:   2,
		}
		if err := tx.Create(member).Error; err != nil {
			return err // 事务异常自动回滚
		}
		return nil // 事务执行成功自动提交
	})
}

func (r *roomRepoImpl) GetRoomByID(roomID int64) (*models.Room, error) {
	var room models.Room
	err := r.db.First(&room, roomID).Error
	return &room, err
}

func (r *roomRepoImpl) SoftDeleteRoom(roomID int64) error {
	// 基于gorm软删除特性，自动转换为更新deleted_at字段
	return r.db.Delete(&models.Room{}, roomID).Error
}

// GetJoinedRooms 联表查询用户加入的群聊，避免N+1查询问题
func (r *roomRepoImpl) GetJoinedRooms(userID int64) ([]*models.Room, error) {
	var rooms []*models.Room
	// 内连接查询，继承软删除规则，一次性获取用户所有群聊
	err := r.db.Model(&models.Room{}).
		Select("rooms.*").
		Joins("INNER JOIN room_members ON rooms.id = room_members.room_id AND room_members.deleted_at = 0").
		Where("room_members.user_id = ?", userID).
		Find(&rooms).Error
	return rooms, err
}

// GetJoinedRoomsWithRole 一次查询返回用户加入的群组及对应角色，避免 N+1。
func (r *roomRepoImpl) GetJoinedRoomsWithRole(userID int64) ([]JoinedRoom, error) {
	var rows []JoinedRoom
	err := r.db.Model(&models.Room{}).
		Select("rooms.id, rooms.name, rooms.agent_enabled, rooms.creator_id, rooms.created_at, room_members.role AS member_role").
		Joins("INNER JOIN room_members ON rooms.id = room_members.room_id AND room_members.deleted_at = 0").
		Where("room_members.user_id = ?", userID).
		Scan(&rows).Error
	return rows, err
}

// GetRoomByExactName 根据群聊名称精确查询群聊
func (r *roomRepoImpl) GetRoomByExactName(exactName string) (*models.Room, error) {
	var room models.Room
	// 采用等值查询，使用索引提升查询效率，不使用模糊查询
	err := r.db.Where("name = ?", exactName).Take(&room).Error
	return &room, err
}
