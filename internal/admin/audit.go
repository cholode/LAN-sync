package admin

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"lan-im-go/models"
)

// AuditService ??????????
type AuditService struct {
	db *gorm.DB
}

// NewAuditService ?????????
func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

// AuditEntry ?????????
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

// Log ????????????????????
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
