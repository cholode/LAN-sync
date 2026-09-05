package admin

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"

	"lan-im-go/cache"
	"lan-im-go/models"
)

// UserService 提供用户管理查询与操作。
type UserService struct {
	db           *gorm.DB
	messageStore MessageStatsStore
	runtime      RuntimeController
	audit        *AuditService
}

// NewUserService 创建用户管理服务。
func NewUserService(db *gorm.DB, messageStore MessageStatsStore, runtime RuntimeController, audit *AuditService) *UserService {
	return &UserService{db: db, messageStore: messageStore, runtime: runtime, audit: audit}
}

// UserListQuery 定义用户列表查询条件。
type UserListQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Role     int8
	Status   int8
	Online   *bool
	Start    time.Time
	End      time.Time
}

// UserListItem 表示用户列表项。
type UserListItem struct {
	ID             int64      `json:"id"`
	Username       string     `json:"username"`
	Role           int8       `json:"role"`
	RoleName       string     `json:"role_name"`
	CreatedAt      time.Time  `json:"created_at"`
	LastLoginAt    *time.Time `json:"last_login_at"`
	LastActiveAt   *time.Time `json:"last_active_at"`
	Online         bool       `json:"online"`
	Status         int8       `json:"status"`
	RoomCount      int64      `json:"room_count"`
	MessageCount   int64      `json:"message_count"`
	ViolationCount int64      `json:"violation_count"`
}

// ListUsers 分页查询用户列表。
// ListUsers 分页查询用户，并用批量查询避免列表页 N+1。
func (s *UserService) ListUsers(ctx context.Context, q UserListQuery) ([]UserListItem, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.User{})
	if q.Keyword != "" {
		if userID, err := strconv.ParseInt(q.Keyword, 10, 64); err == nil {
			query = query.Where("id = ?", userID)
		} else {
			query = query.Where("username LIKE ?", q.Keyword+"%")
		}
	}
	if q.Role >= 0 {
		query = query.Where("role = ?", q.Role)
	}
	if q.Status >= 0 {
		query = query.Where("status = ?", q.Status)
	}
	if !q.Start.IsZero() {
		query = query.Where("created_at >= ?", q.Start)
	}
	if !q.End.IsZero() {
		query = query.Where("created_at < ?", q.End)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []models.User
	if err := query.Order("id DESC").
		Offset((q.Page - 1) * q.PageSize).
		Limit(q.PageSize).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	userIDs := make([]int64, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}

	onlineMap, _ := cache.CheckUsersOnline(ctx, userIDs)
	roomCountMap, _ := s.countUserRoomsByIDs(ctx, userIDs)
	messageCountMap, _ := s.messageStore.CountsBySenderIDs(ctx, userIDs)
	violationCountMap, _ := s.countUserViolationsByIDs(ctx, userIDs)

	items := make([]UserListItem, 0, len(users))
	for _, user := range users {
		item := UserListItem{
			ID:             user.ID,
			Username:       user.Username,
			Role:           user.Role,
			RoleName:       models.RoleName(user.Role),
			CreatedAt:      user.CreatedAt,
			LastLoginAt:    user.LastLoginAt,
			LastActiveAt:   user.LastActiveAt,
			Online:         onlineMap[user.ID],
			Status:         user.Status,
			RoomCount:      roomCountMap[user.ID],
			MessageCount:   messageCountMap[user.ID],
			ViolationCount: violationCountMap[user.ID],
		}
		items = append(items, item)
	}

	return items, total, nil
}

// UserDetail 表示用户详情。
type UserDetail struct {
	ID             int64                 `json:"id"`
	Username       string                `json:"username"`
	Role           int8                  `json:"role"`
	RoleName       string                `json:"role_name"`
	Status         int8                  `json:"status"`
	CreatedAt      time.Time             `json:"created_at"`
	LastLoginAt    *time.Time            `json:"last_login_at"`
	LastActiveAt   *time.Time            `json:"last_active_at"`
	Online         bool                  `json:"online"`
	RoomCount      int64                 `json:"room_count"`
	MessageCount   int64                 `json:"message_count"`
	ViolationCount int64                 `json:"violation_count"`
	Rooms          []UserRoomItem        `json:"rooms"`
	RecentMessages []string              `json:"recent_messages"`
	Violations     []ModerationEventItem `json:"violations"`
	FileUploads    []any                 `json:"file_uploads"`
	AgentCalls     []any                 `json:"agent_calls"`
}

type UserRoomItem struct {
	RoomID   int64  `json:"room_id"`
	RoomName string `json:"room_name"`
	Role     int8   `json:"role"`
}

// GetUserDetail 获取用户详情。
func (s *UserService) GetUserDetail(ctx context.Context, userID int64) (*UserDetail, error) {
	var user models.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, err
	}
	online, _, _ := cache.CheckUserOnline(ctx, user.ID)
	detail := &UserDetail{
		ID:             user.ID,
		Username:       user.Username,
		Role:           user.Role,
		RoleName:       models.RoleName(user.Role),
		Status:         user.Status,
		CreatedAt:      user.CreatedAt,
		LastLoginAt:    user.LastLoginAt,
		LastActiveAt:   user.LastActiveAt,
		Online:         online,
		Rooms:          []UserRoomItem{},
		RecentMessages: []string{},
		Violations:     []ModerationEventItem{},
		FileUploads:    []any{},
		AgentCalls:     []any{},
	}
	detail.RoomCount, _ = s.countUserRooms(ctx, user.ID)
	detail.MessageCount, _ = s.messageStore.CountBySender(ctx, user.ID)
	detail.ViolationCount, _ = s.countUserViolations(ctx, user.ID)
	detail.Rooms, _ = s.userRooms(ctx, user.ID)

	var violations []models.ModerationEvent
	_ = s.db.WithContext(ctx).Where("user_id = ?", user.ID).Order("created_at DESC").Limit(20).Find(&violations).Error
	detail.Violations = moderationItems(violations)

	return detail, nil
}

// UserAction 表示用户管理操作。
type UserAction struct {
	Action      string
	AdminUserID int64
	AdminName   string
	RequestID   string
	RemoteIP    string
	UserAgent   string
}

// ApplyAction 执行封禁、解封、角色调整、强制下线等用户管理动作。
func (s *UserService) ApplyAction(ctx context.Context, userID int64, action UserAction) error {
	var user models.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return err
	}
	before := user
	switch action.Action {
	case "ban":
		user.Status = 1
	case "unban":
		user.Status = 0
	case "role_super_admin":
		user.Role = models.RoleSuperAdmin
	case "role_moderator":
		user.Role = models.RoleModerator
	case "role_operator":
		user.Role = models.RoleOperator
	case "role_user":
		user.Role = models.RoleUser
	default:
		return fmt.Errorf("不支持的用户操作: %s", action.Action)
	}

	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return err
	}
	if action.Action == "ban" && s.runtime != nil {
		if err := s.runtime.KickUser(ctx, userID); err != nil {
			return err
		}
		_ = cache.SetUserOffline(ctx, userID)
	}
	return s.writeUserAudit(ctx, before, user, action)
}

func (s *UserService) writeUserAudit(ctx context.Context, before, after models.User, action UserAction) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.Log(ctx, AuditEntry{
		AdminUserID:   action.AdminUserID,
		AdminUsername: action.AdminName,
		Action:        "user." + action.Action,
		TargetType:    "user",
		TargetID:      strconv.FormatInt(before.ID, 10),
		BeforeData:    userAuditItem(before),
		AfterData:     userAuditItem(after),
		RequestID:     action.RequestID,
		RemoteIP:      action.RemoteIP,
		UserAgent:     action.UserAgent,
		Result:        "success",
	})
}

func userAuditItem(user models.User) map[string]any {
	return map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
		"status":   user.Status,
	}
}

func (s *UserService) countUserRoomsByIDs(ctx context.Context, userIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		UserID int64
		Count  int64
	}
	err := s.db.WithContext(ctx).Model(&models.RoomMember{}).
		Select("user_id, COUNT(*) AS count").
		Where("user_id IN ?", userIDs).
		Group("user_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.UserID] = row.Count
	}
	return out, nil
}

func (s *UserService) countUserViolationsByIDs(ctx context.Context, userIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		UserID int64
		Count  int64
	}
	err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).
		Select("user_id, COUNT(*) AS count").
		Where("user_id IN ?", userIDs).
		Group("user_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.UserID] = row.Count
	}
	return out, nil
}

func (s *UserService) countUserRooms(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.RoomMember{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (s *UserService) countUserViolations(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (s *UserService) userRooms(ctx context.Context, userID int64) ([]UserRoomItem, error) {
	type row struct {
		RoomID   int64
		RoomName string
		Role     int8
	}
	var rows []row
	err := s.db.WithContext(ctx).
		Model(&models.RoomMember{}).
		Select("room_members.room_id, rooms.name AS room_name, room_members.role").
		Joins("INNER JOIN rooms ON rooms.id = room_members.room_id AND rooms.deleted_at = 0").
		Where("room_members.user_id = ?", userID).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]UserRoomItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, UserRoomItem{RoomID: r.RoomID, RoomName: r.RoomName, Role: r.Role})
	}
	return out, nil
}
