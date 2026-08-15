package admin

import (
	"context"
	"strconv"

	"lan-im-go/core"
)

// ConnectionService ?????????
type ConnectionService struct {
	hub   *core.Hub
	audit *AuditService
}

// NewConnectionService ???????????
func NewConnectionService(hub *core.Hub, audit *AuditService) *ConnectionService {
	return &ConnectionService{hub: hub, audit: audit}
}

// ListConnections ???????????????????/??ID???
func (s *ConnectionService) ListConnections(ctx context.Context, keyword string) []core.ConnectionSnapshot {
	if s.hub == nil {
		return []core.ConnectionSnapshot{}
	}
	snapshots := s.hub.Connections()
	if keyword == "" {
		return snapshots
	}
	out := make([]core.ConnectionSnapshot, 0, len(snapshots))
	for _, item := range snapshots {
		if strconv.FormatInt(item.UserID, 10) == keyword || item.Username == keyword {
			out = append(out, item)
		}
	}
	return out
}

// CloseConnection ?????????
func (s *ConnectionService) CloseConnection(ctx context.Context, connectionID string, action ConnectionAction) error {
	if s.hub != nil {
		s.hub.CloseConnection(connectionID)
	}
	if s.audit != nil {
		return s.audit.Log(ctx, AuditEntry{
			AdminUserID:   action.AdminUserID,
			AdminUsername: action.AdminName,
			Action:        "connection.close",
			TargetType:    "connection",
			TargetID:      connectionID,
			RequestID:     action.RequestID,
			RemoteIP:      action.RemoteIP,
			UserAgent:     action.UserAgent,
			Result:        "success",
		})
	}
	return nil
}

// ForceOffline ?????????
func (s *ConnectionService) ForceOffline(ctx context.Context, userID int64, action ConnectionAction) error {
	if s.hub != nil {
		select {
		case s.hub.Kick <- userID:
		default:
		}
	}
	if s.audit != nil {
		return s.audit.Log(ctx, AuditEntry{
			AdminUserID:   action.AdminUserID,
			AdminUsername: action.AdminName,
			Action:        "connection.force_offline",
			TargetType:    "user",
			TargetID:      strconv.FormatInt(userID, 10),
			RequestID:     action.RequestID,
			RemoteIP:      action.RemoteIP,
			UserAgent:     action.UserAgent,
			Result:        "success",
		})
	}
	return nil
}

// ConnectionAction ??????????
type ConnectionAction struct {
	AdminUserID int64
	AdminName   string
	RequestID   string
	RemoteIP    string
	UserAgent   string
}
