package admin

import (
	"context"
	"strconv"
	"time"

	"gorm.io/gorm"

	"lan-im-go/cache"
	"lan-im-go/core"
	"lan-im-go/models"
)

// UserService ?????????
type UserService struct {
	db           *gorm.DB
	messageStore MessageStatsStore
	hub          *core.Hub
	audit        *AuditService
}

// NewUserService ?????????
func NewUserService(db *gorm.DB, messageStore MessageStatsStore, hub *core.Hub, audit *AuditService) *UserService {
	return &UserService{db: db, messageStore: messageStore, hub: hub, audit: audit}
}

// UserListQuery ?????????
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

// UserListItem ??????
type UserListItem struct {
	ID             int64     `json:"id"`
	Username       string    `json:"username"`
	Role           int8      `json:"role"`
	RoleName       string    `json:"role_name"`
	CreatedAt      time.Time `json:"created_at"`
	LastLoginAt    time.Time `json:"last_login_at"`
	LastActiveAt   time.Time `json:"last_active_at"`
	Online         bool      `json:"online"`
	Status         int8      `json:"status"`
	RoomCount      int64     `json:"room_count"`
	MessageCount   int64     `json:"message_count"`
	ViolationCount int64     `json:"violation_count"`
}

// ListUsers ???????
func (s *UserService) ListUsers(ctx context.Context, q UserListQuery) ([]UserListItem, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.User{})
	if q.Keyword != "" {
		query = query.Where("username LIKE ?", "%"+q.Keyword+"%")
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

	items := make([]UserListItem, 0, len(users))
	for _, user := range users {
		online, _, _ := cache.CheckUserOnline(ctx, user.ID)
		item := UserListItem{
			ID:           user.ID,
			Username:     user.Username,
			Role:         user.Role,
			RoleName:     models.RoleName(user.Role),
			CreatedAt:    user.CreatedAt,
			LastLoginAt:  user.LastLoginAt,
			LastActiveAt: user.LastActiveAt,
			Online:       online,
			Status:       user.Status,
		}
		item.RoomCount, _ = s.countUserRooms(ctx, user.ID)
		item.MessageCount, _ = s.messageStore.CountBySender(ctx, user.ID)
		item.ViolationCount, _ = s.countUserViolations(ctx, user.ID)
		items = append(items, item)
	}

	return items, total, nil
}

// UserDetail ?????
type UserDetail struct {
	ID             int64                 `json:"id"`
	Username       string                `json:"username"`
	Role           int8                  `json:"role"`
	RoleName       string                `json:"role_name"`
	Status         int8                  `json:"status"`
	CreatedAt      time.Time             `json:"created_at"`
	LastLoginAt    time.Time             `json:"last_login_at"`
	LastActiveAt   time.Time             `json:"last_active_at"`
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

// GetUserDetail ???????
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

// UserAction ???????
type UserAction struct {
	Action      string
	AdminUserID int64
	AdminName   string
	RequestID   string
	RemoteIP    string
	UserAgent   string
}

// ApplyAction ???????/??/???/????????????
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
	case "force_offline":
		if s.hub != nil {
			select {
			case s.hub.Kick <- userID:
			default:
			}
		}
		_ = cache.SetUserOffline(ctx, userID)
		after := user
		return s.writeUserAudit(ctx, before, after, action)
	default:
		return nil
	}

	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return err
	}
	if action.Action == "ban" && s.hub != nil {
		select {
		case s.hub.Kick <- userID:
		default:
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
