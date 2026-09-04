package admin

import (
	"context"

	"lan-im-go/services/gateway/websocket"
	"lan-im-go/shared/observability/metrics"
)

// RuntimeController 抽象管理端依赖的 IM 主服务运行时控制能力。
// 主服务进程使用本地 Hub 实现，独立管理端进程使用 gRPC 客户端实现。
type RuntimeController interface {
	// ListConnections 获取当前节点 WebSocket 连接快照。
	ListConnections(ctx context.Context) ([]core.ConnectionSnapshot, error)
	// CloseConnection 关闭指定连接。
	CloseConnection(ctx context.Context, connectionID string) error
	// KickUser 强制用户下线。
	KickUser(ctx context.Context, userID int64) error
	// DisbandRoom 通知 Hub 解散群聊。
	DisbandRoom(ctx context.Context, roomID int64) error
	// RemoveRoomMember 通知 Hub 将成员移出群聊。
	RemoveRoomMember(ctx context.Context, roomID, userID int64) error
	// HubStats 获取当前节点连接与房间数量。
	HubStats(ctx context.Context) (core.HubStats, error)
	// RuntimeSnapshots 获取运行时与 Agent 指标快照。
	RuntimeSnapshots(ctx context.Context) (metrics.RuntimeSnapshot, metrics.AgentRuntimeSnapshot, error)
}
