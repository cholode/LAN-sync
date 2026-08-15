package admin

import (
	"context"
	"regexp"
	"time"

	"gorm.io/gorm"

	"lan-im-go/models"
)

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)(password|passwd|pwd)(\s*[:=]\s*)[^\s,;]+`),
	regexp.MustCompile(`(?i)(api[_-]?key|secret[_-]?key|access[_-]?key)(\s*[:=]\s*)[^\s,;]+`),
	regexp.MustCompile(`(?i)(authorization)(\s*[:=]\s*)[^\s,;]+`),
}

// ErrorCenterService \u7ba1\u7406\u7cfb\u7edf\u9519\u8bef\u4e2d\u5fc3\u3002
type ErrorCenterService struct {
	db *gorm.DB
}

func NewErrorCenterService(db *gorm.DB) *ErrorCenterService {
	return &ErrorCenterService{db: db}
}

// ErrorListQuery \u7cfb\u7edf\u9519\u8bef\u67e5\u8be2\u6761\u4ef6\u3002
type ErrorListQuery struct {
	Page      int
	PageSize  int
	Module    string
	ErrorType string
	Resolved  *bool
	Start     time.Time
	End       time.Time
}

// RecordErrorInput \u5199\u5165\u9519\u8bef\u65e5\u5fd7\u7684\u5165\u53c2\u3002
type RecordErrorInput struct {
	Module       string
	ErrorType    string
	ErrorMessage string
	RequestID    string
	UserID       int64
	RoomID       int64
	StackTrace   string
}

// List \u5206\u9875\u67e5\u8be2\u7cfb\u7edf\u9519\u8bef\u3002
func (s *ErrorCenterService) List(ctx context.Context, q ErrorListQuery) ([]models.SystemErrorLog, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.SystemErrorLog{})
	if q.Module != "" {
		query = query.Where("module = ?", q.Module)
	}
	if q.ErrorType != "" {
		query = query.Where("error_type = ?", q.ErrorType)
	}
	if q.Resolved != nil {
		query = query.Where("resolved = ?", *q.Resolved)
	}
	if !q.Start.IsZero() {
		query = query.Where("timestamp >= ?", q.Start)
	}
	if !q.End.IsZero() {
		query = query.Where("timestamp < ?", q.End)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []models.SystemErrorLog
	if err := query.Order("id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// Record \u5199\u5165\u4e00\u6761\u5df2\u8131\u654f\u7684\u7cfb\u7edf\u9519\u8bef\u3002
func (s *ErrorCenterService) Record(ctx context.Context, input RecordErrorInput) error {
	record := &models.SystemErrorLog{
		Timestamp:    time.Now(),
		Module:       input.Module,
		ErrorType:    input.ErrorType,
		ErrorMessage: sanitizeErrorText(input.ErrorMessage),
		RequestID:    input.RequestID,
		UserID:       input.UserID,
		RoomID:       input.RoomID,
		StackTrace:   sanitizeErrorText(input.StackTrace),
		Resolved:     false,
	}
	return s.db.WithContext(ctx).Create(record).Error
}

// Resolve \u6807\u8bb0\u9519\u8bef\u5df2\u5904\u7406\u3002
func (s *ErrorCenterService) Resolve(ctx context.Context, id, adminUserID int64) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.SystemErrorLog{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"resolved":    true,
			"resolved_by": adminUserID,
			"resolved_at": now,
		}).Error
}

// Summary \u7edf\u8ba1\u672a\u5904\u7406\u9519\u8bef\u6570\u91cf\u3002
func (s *ErrorCenterService) Summary(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.SystemErrorLog{}).Where("resolved = ?", false).Count(&count).Error
	return count, err
}

func sanitizeErrorText(value string) string {
	if value == "" {
		return ""
	}
	for _, pattern := range sensitivePatterns {
		value = pattern.ReplaceAllString(value, "$1[REDACTED]")
	}
	return value
}
