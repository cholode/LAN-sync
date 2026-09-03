package admin

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"lan-im-go/models"
)

// AuditService 负责记录管理员操作审计日志。
type AuditService struct {
	db *gorm.DB
}

// NewAuditService 创建审计服务实例。
func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

// AuditEntry 表示单条审计记录。
type AuditEntry struct {
	AdminUserID   int64
	AdminUsername string
	Action        string
	TargetType    string
	TargetID      string
	BeforeData    any
	AfterData     any
	RequestID     string
	RemoteIP      string
	UserAgent     string
	Result        string
	ErrorMessage  string
}

// Log 将审计事件写入数据库。
// AuditListQuery 审计日志查询条件。
type AuditListQuery struct {
	Page        int
	PageSize    int
	Keyword     string
	AdminUserID int64
	Action      string
	TargetType  string
	TargetID    string
	Result      string
	Start       time.Time
	End         time.Time
}

// List 分页查询审计日志。
func (s *AuditService) List(ctx context.Context, q AuditListQuery) ([]models.AdminAuditLog, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.AdminAuditLog{})
	if q.Keyword != "" {
		like := q.Keyword + "%"
		query = query.Where("action LIKE ? OR target_type LIKE ? OR target_id LIKE ? OR admin_username LIKE ?", like, like, like, like)
	}
	if q.AdminUserID > 0 {
		query = query.Where("admin_user_id = ?", q.AdminUserID)
	}
	if q.Action != "" {
		query = query.Where("action = ?", q.Action)
	}
	if q.TargetType != "" {
		query = query.Where("target_type = ?", q.TargetType)
	}
	if q.TargetID != "" {
		query = query.Where("target_id = ?", q.TargetID)
	}
	if q.Result != "" {
		query = query.Where("result = ?", q.Result)
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

	var rows []models.AdminAuditLog
	if err := query.Order("id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *AuditService) Log(ctx context.Context, entry AuditEntry) error {
	before := marshalAuditData(entry.BeforeData)
	after := marshalAuditData(entry.AfterData)
	record := &models.AdminAuditLog{
		AdminUserID:   entry.AdminUserID,
		AdminUsername: entry.AdminUsername,
		Action:        entry.Action,
		TargetType:    entry.TargetType,
		TargetID:      entry.TargetID,
		BeforeData:    before,
		AfterData:     after,
		RequestID:     entry.RequestID,
		RemoteIP:      entry.RemoteIP,
		UserAgent:     entry.UserAgent,
		Result:        entry.Result,
		ErrorMessage:  entry.ErrorMessage,
	}
	return s.db.WithContext(ctx).Create(record).Error
}

func marshalAuditData(data any) string {
	if data == nil {
		return ""
	}
	switch value := data.(type) {
	case string:
		return value
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(raw)
	}
}
