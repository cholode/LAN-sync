package admin

import (
	"context"
	"strconv"

	"lan-im-go/services/gateway/websocket"
)

// ConnectionService 管理当前节点的 WebSocket 连接。
type ConnectionService struct {
	runtime RuntimeController
	audit   *AuditService
}

// NewConnectionService 创建连接管理服务。
func NewConnectionService(runtime RuntimeController, audit *AuditService) *ConnectionService {
	return &ConnectionService{runtime: runtime, audit: audit}
}

// ListConnections 返回连接快照，并支持按用户 ID 或用户名过滤。
func (s *ConnectionService) ListConnections(ctx context.Context, keyword string) []core.ConnectionSnapshot {
	if s.runtime == nil {
		return []core.ConnectionSnapshot{}
	}
	snapshots, err := s.runtime.ListConnections(ctx)
	if err != nil {
		return []core.ConnectionSnapshot{}
	}
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

// CloseConnection 关闭指定 WebSocket 连接。
func (s *ConnectionService) CloseConnection(ctx context.Context, connectionID string, action ConnectionAction) error {
	if s.runtime != nil {
		if err := s.runtime.CloseConnection(ctx, connectionID); err != nil {
			return err
		}
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

// ForceOffline 强制指定用户下线。
func (s *ConnectionService) ForceOffline(ctx context.Context, userID int64, action ConnectionAction) error {
	if s.runtime != nil {
		if err := s.runtime.KickUser(ctx, userID); err != nil {
			return err
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

// ConnectionAction 连接操作的审计上下文。
type ConnectionAction struct {
	AdminUserID int64
	AdminName   string
	RequestID   string
	RemoteIP    string
	UserAgent   string
}
