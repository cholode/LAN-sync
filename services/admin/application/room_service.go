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

// RoomService 提供房间管理查询与操作。
type RoomService struct {
	db           *gorm.DB
	messageStore MessageStatsStore
	runtime      RuntimeController
	audit        *AuditService
}

// NewRoomService 创建房间管理服务。
func NewRoomService(db *gorm.DB, messageStore MessageStatsStore, runtime RuntimeController, audit *AuditService) *RoomService {
	return &RoomService{db: db, messageStore: messageStore, runtime: runtime, audit: audit}
}

// RoomListQuery 定义房间列表查询条件。
type RoomListQuery struct {
	Page         int
	PageSize     int
	Keyword      string
	RoomType     int8
	AgentEnabled *bool
	Status       int8
	Start        time.Time
	End          time.Time
}

// RoomListItem 表示房间列表项。
type RoomListItem struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	Type              int8      `json:"type"`
	OwnerID           int64     `json:"owner_id"`
	MemberCount       int64     `json:"member_count"`
	OnlineMemberCount int64     `json:"online_member_count"`
	TodayMessageCount int64     `json:"today_message_count"`
	TotalMessageCount int64     `json:"total_message_count"`
	CreatedAt         time.Time `json:"created_at"`
	LastActiveAt      time.Time `json:"last_active_at"`
	AgentEnabled      bool      `json:"agent_enabled"`
	ModerationEnabled bool      `json:"moderation_enabled"`
	Status            int8      `json:"status"`
	ViolationCount    int64     `json:"violation_count"`
}

// ListRooms 分页查询房间列表。
// ListRooms 分页查询群聊，并用批量查询避免列表页 N+1。
func (s *RoomService) ListRooms(ctx context.Context, q RoomListQuery) ([]RoomListItem, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.Room{})
	if q.Keyword != "" {
		if roomID, err := strconv.ParseInt(q.Keyword, 10, 64); err == nil {
			query = query.Where("id = ?", roomID)
		} else {
			query = query.Where("name LIKE ?", q.Keyword+"%")
		}
	}
	if q.RoomType > 0 {
		query = query.Where("type = ?", q.RoomType)
	}
	if q.AgentEnabled != nil {
		query = query.Where("agent_enabled = ?", *q.AgentEnabled)
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

	var rooms []models.Room
	if err := query.Order("id DESC").
		Offset((q.Page - 1) * q.PageSize).
		Limit(q.PageSize).
		Find(&rooms).Error; err != nil {
		return nil, 0, err
	}

	roomIDs := make([]int64, 0, len(rooms))
	for _, room := range rooms {
		roomIDs = append(roomIDs, room.ID)
	}

	start := startOfDay(time.Now())
	now := time.Now()
	memberCountMap, _ := s.countMembersByIDs(ctx, roomIDs)
	onlineCountMap, _ := s.onlineMemberCounts(ctx, roomIDs)
	todayMessageMap, _ := s.messageStore.CountsByRoomIDs(ctx, roomIDs, start, now)
	totalMessageMap, _ := s.messageStore.CountsByRoomTotalIDs(ctx, roomIDs)
	violationCountMap, _ := s.countRoomViolationsByIDs(ctx, roomIDs, start)

	items := make([]RoomListItem, 0, len(rooms))
	for _, room := range rooms {
		item := RoomListItem{
			ID:                room.ID,
			Name:              room.Name,
			Type:              room.Type,
			OwnerID:           room.CreatorID,
			CreatedAt:         room.CreatedAt,
			LastActiveAt:      room.LastActiveAt,
			AgentEnabled:      room.AgentEnabled,
			ModerationEnabled: room.ModerationEnabled,
			Status:            room.Status,
			MemberCount:       memberCountMap[room.ID],
			OnlineMemberCount: onlineCountMap[room.ID],
			TodayMessageCount: todayMessageMap[room.ID],
			TotalMessageCount: totalMessageMap[room.ID],
			ViolationCount:    violationCountMap[room.ID],
		}
		items = append(items, item)
	}
	return items, total, nil
}

// RoomDetail 表示房间详情。
type RoomDetail struct {
	ID                int64                 `json:"id"`
	Name              string                `json:"name"`
	Type              int8                  `json:"type"`
	OwnerID           int64                 `json:"owner_id"`
	AgentEnabled      bool                  `json:"agent_enabled"`
	ModerationEnabled bool                  `json:"moderation_enabled"`
	Status            int8                  `json:"status"`
	CreatedAt         time.Time             `json:"created_at"`
	LastActiveAt      time.Time             `json:"last_active_at"`
	MemberCount       int64                 `json:"member_count"`
	OnlineMemberCount int64                 `json:"online_member_count"`
	TodayMessageCount int64                 `json:"today_message_count"`
	TotalMessageCount int64                 `json:"total_message_count"`
	ViolationCount    int64                 `json:"violation_count"`
	Members           []RoomMemberItem      `json:"members"`
	RecentMessages    []string              `json:"recent_messages"`
	AgentConfig       any                   `json:"agent_config"`
	Violations        []ModerationEventItem `json:"violations"`
}

// RoomMemberItem 表示房间成员项。
type RoomMemberItem struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     int8   `json:"role"`
	Online   bool   `json:"online"`
}

// GetRoomDetail 获取房间详情。
func (s *RoomService) GetRoomDetail(ctx context.Context, roomID int64) (*RoomDetail, error) {
	var room models.Room
	if err := s.db.WithContext(ctx).First(&room, roomID).Error; err != nil {
		return nil, err
	}
	start := startOfDay(time.Now())
	detail := &RoomDetail{
		ID:                room.ID,
		Name:              room.Name,
		Type:              room.Type,
		OwnerID:           room.CreatorID,
		AgentEnabled:      room.AgentEnabled,
		ModerationEnabled: room.ModerationEnabled,
		Status:            room.Status,
		CreatedAt:         room.CreatedAt,
		LastActiveAt:      room.LastActiveAt,
		Members:           []RoomMemberItem{},
		RecentMessages:    []string{},
		Violations:        []ModerationEventItem{},
	}
	detail.MemberCount, _ = s.countMembers(ctx, room.ID)
	detail.OnlineMemberCount, _ = s.countOnlineMembers(ctx, room.ID)
	detail.TodayMessageCount, _ = s.messageStore.CountByRoom(ctx, room.ID, start, time.Now())
	detail.TotalMessageCount, _ = s.messageStore.CountByRoomTotal(ctx, room.ID)
	detail.ViolationCount, _ = s.countRoomViolations(ctx, room.ID, start)
	detail.Members, _ = s.roomMembers(ctx, room.ID)

	var cfg models.AgentConfig
	if err := s.db.WithContext(ctx).Where("room_id = ?", room.ID).First(&cfg).Error; err == nil {
		detail.AgentConfig = cfg
	} else {
		detail.AgentConfig = models.DefaultAgentConfig(room.ID)
	}

	var violations []models.ModerationEvent
	_ = s.db.WithContext(ctx).Where("room_id = ?", room.ID).Order("created_at DESC").Limit(20).Find(&violations).Error
	detail.Violations = moderationItems(violations)

	return detail, nil
}

// RoomAction 表示房间管理操作。
type RoomAction struct {
	Action       string
	AdminUserID  int64
	AdminName    string
	RequestID    string
	RemoteIP     string
	UserAgent    string
	TargetUserID int64
}

// ApplyAction 执行冻结、解散、成员管理等房间管理动作。
func (s *RoomService) ApplyAction(ctx context.Context, roomID int64, action RoomAction) error {
	var room models.Room
	if err := s.db.WithContext(ctx).First(&room, roomID).Error; err != nil {
		return err
	}
	before := room

	switch action.Action {
	case "freeze":
		room.Status = 1
	case "unfreeze":
		room.Status = 0
	case "disband":
		if err := s.db.WithContext(ctx).Delete(&models.Room{}, roomID).Error; err != nil {
			return err
		}
		if s.runtime != nil {
			if err := s.runtime.DisbandRoom(ctx, roomID); err != nil {
				return err
			}
		}
		return s.writeRoomAudit(ctx, before, room, action)
	case "agent_enable":
		return fmt.Errorf("群聊 Agent 已迁移到 Python Agent 服务，请通过绑定接口启用")
	case "agent_disable":
		return fmt.Errorf("群聊 Agent 已迁移到 Python Agent 服务，请通过绑定接口停用")
	case "moderation_enable":
		room.ModerationEnabled = true
	case "moderation_disable":
		room.ModerationEnabled = false
	case "remove_member":
		if action.TargetUserID > 0 {
			if err := s.db.WithContext(ctx).Where("room_id = ? AND user_id = ?", roomID, action.TargetUserID).Delete(&models.RoomMember{}).Error; err != nil {
				return err
			}
			if s.runtime != nil {
				if err := s.runtime.RemoveRoomMember(ctx, roomID, action.TargetUserID); err != nil {
					return err
				}
			}
			return s.writeRoomAudit(ctx, before, room, action)
		}
	case "set_admin":
		if action.TargetUserID > 0 {
			return s.updateMemberRole(ctx, roomID, action.TargetUserID, 2, action)
		}
	case "transfer_owner":
		if action.TargetUserID > 0 {
			room.CreatorID = action.TargetUserID
			if err := s.db.WithContext(ctx).Save(&room).Error; err != nil {
				return err
			}
			_ = s.updateMemberRole(ctx, roomID, action.TargetUserID, 3, action)
			return s.writeRoomAudit(ctx, before, room, action)
		}
	default:
		return fmt.Errorf("不支持的群聊操作: %s", action.Action)
	}

	if err := s.db.WithContext(ctx).Save(&room).Error; err != nil {
		return err
	}
	return s.writeRoomAudit(ctx, before, room, action)
}

func (s *RoomService) updateMemberRole(ctx context.Context, roomID, userID int64, role int8, action RoomAction) error {
	err := s.db.WithContext(ctx).Model(&models.RoomMember{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Update("role", role).Error
	if err != nil {
		return err
	}
	room := models.Room{ID: roomID}
	return s.writeRoomAudit(ctx, room, room, action)
}

func (s *RoomService) writeRoomAudit(ctx context.Context, before, after models.Room, action RoomAction) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.Log(ctx, AuditEntry{
		AdminUserID:   action.AdminUserID,
		AdminUsername: action.AdminName,
		Action:        "room." + action.Action,
		TargetType:    "room",
		TargetID:      strconv.FormatInt(after.ID, 10),
		BeforeData:    roomAuditItem(before),
		AfterData:     roomAuditItem(after),
		RequestID:     action.RequestID,
		RemoteIP:      action.RemoteIP,
		UserAgent:     action.UserAgent,
		Result:        "success",
	})
}

func roomAuditItem(room models.Room) map[string]any {
	return map[string]any{
		"id":                 room.ID,
		"name":               room.Name,
		"agent_enabled":      room.AgentEnabled,
		"moderation_enabled": room.ModerationEnabled,
		"status":             room.Status,
	}
}

func (s *RoomService) countMembersByIDs(ctx context.Context, roomIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(roomIDs))
	if len(roomIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		RoomID int64
		Count  int64
	}
	err := s.db.WithContext(ctx).Model(&models.RoomMember{}).
		Select("room_id, COUNT(*) AS count").
		Where("room_id IN ?", roomIDs).
		Group("room_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.RoomID] = row.Count
	}
	return out, nil
}

func (s *RoomService) onlineMemberCounts(ctx context.Context, roomIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(roomIDs))
	if len(roomIDs) == 0 {
		return out, nil
	}
	var members []models.RoomMember
	if err := s.db.WithContext(ctx).Where("room_id IN ?", roomIDs).Find(&members).Error; err != nil {
		return nil, err
	}

	userIDs := make([]int64, 0, len(members))
	seen := make(map[int64]struct{}, len(members))
	for _, member := range members {
		if _, ok := seen[member.UserID]; !ok {
			seen[member.UserID] = struct{}{}
			userIDs = append(userIDs, member.UserID)
		}
	}
	onlineMap, _ := cache.CheckUsersOnline(ctx, userIDs)
	for _, member := range members {
		if onlineMap[member.UserID] {
			out[member.RoomID]++
		}
	}
	return out, nil
}

func (s *RoomService) countRoomViolationsByIDs(ctx context.Context, roomIDs []int64, since time.Time) (map[int64]int64, error) {
	out := make(map[int64]int64, len(roomIDs))
	if len(roomIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		RoomID int64
		Count  int64
	}
	err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).
		Select("room_id, COUNT(*) AS count").
		Where("room_id IN ? AND created_at >= ?", roomIDs, since).
		Group("room_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.RoomID] = row.Count
	}
	return out, nil
}

func (s *RoomService) countMembers(ctx context.Context, roomID int64) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.RoomMember{}).Where("room_id = ?", roomID).Count(&count).Error
	return count, err
}

func (s *RoomService) countOnlineMembers(ctx context.Context, roomID int64) (int64, error) {
	members, err := s.roomMembers(ctx, roomID)
	if err != nil {
		return 0, err
	}
	var online int64
	for _, member := range members {
		if member.Online {
			online++
		}
	}
	return online, nil
}

func (s *RoomService) roomMembers(ctx context.Context, roomID int64) ([]RoomMemberItem, error) {
	var members []models.RoomMember
	if err := s.db.WithContext(ctx).Where("room_id = ?", roomID).Find(&members).Error; err != nil {
		return nil, err
	}

	userIDs := make([]int64, 0, len(members))
	seen := make(map[int64]struct{}, len(members))
	for _, member := range members {
		if _, ok := seen[member.UserID]; !ok {
			seen[member.UserID] = struct{}{}
			userIDs = append(userIDs, member.UserID)
		}
	}

	var users []models.User
	if err := s.db.WithContext(ctx).Select("id, username").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	usernameMap := make(map[int64]string, len(users))
	for _, user := range users {
		usernameMap[user.ID] = user.Username
	}
	onlineMap, _ := cache.CheckUsersOnline(ctx, userIDs)

	out := make([]RoomMemberItem, 0, len(members))
	for _, member := range members {
		out = append(out, RoomMemberItem{
			UserID:   member.UserID,
			Username: usernameMap[member.UserID],
			Role:     member.Role,
			Online:   onlineMap[member.UserID],
		})
	}
	return out, nil
}

func (s *RoomService) countRoomViolations(ctx context.Context, roomID int64, since time.Time) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).
		Where("room_id = ? AND created_at >= ?", roomID, since).
		Count(&count).Error
	return count, err
}
