package admincontrol

import (
	"context"
	"fmt"

	"lan-im-go/agent"
	"lan-im-go/core"
	"lan-im-go/internal/metrics"
)

// LocalRuntimeController 在主 IM 服务进程内直接操作 Hub 与 AgentManager。
type LocalRuntimeController struct {
	Hub          *core.Hub
	AgentManager *agent.AgentManager
}

func (c *LocalRuntimeController) ListConnections(_ context.Context) ([]core.ConnectionSnapshot, error) {
	if c.Hub == nil {
		return []core.ConnectionSnapshot{}, nil
	}
	return c.Hub.Connections(), nil
}

func (c *LocalRuntimeController) CloseConnection(_ context.Context, connectionID string) error {
	if c.Hub == nil {
		return fmt.Errorf("Hub 未初始化")
	}
	c.Hub.CloseConnection(connectionID)
	return nil
}

func (c *LocalRuntimeController) KickUser(_ context.Context, userID int64) error {
	if c.Hub == nil {
		return fmt.Errorf("Hub 未初始化")
	}
	c.Hub.Kick(userID)
	return nil
}

func (c *LocalRuntimeController) DisbandRoom(_ context.Context, roomID int64) error {
	if c.Hub == nil {
		return fmt.Errorf("Hub 未初始化")
	}
	c.Hub.DisbandRoom(roomID)
	return nil
}

func (c *LocalRuntimeController) RemoveRoomMember(_ context.Context, roomID, userID int64) error {
	if c.Hub == nil {
		return fmt.Errorf("Hub 未初始化")
	}
	c.Hub.LeaveRoom(userID, roomID)
	return nil
}

func (c *LocalRuntimeController) HubStats(_ context.Context) (core.HubStats, error) {
	if c.Hub == nil {
		return core.HubStats{}, fmt.Errorf("Hub 未初始化")
	}
	return c.Hub.Stats(), nil
}

func (c *LocalRuntimeController) RuntimeSnapshots(_ context.Context) (metrics.RuntimeSnapshot, metrics.AgentRuntimeSnapshot, error) {
	return metrics.RuntimeSnapshotNow(), metrics.AgentRuntimeSnapshotNow(), nil
}

func (c *LocalRuntimeController) AddAgent(ctx context.Context, roomID int64) error {
	if c.AgentManager == nil {
		return fmt.Errorf("AgentManager 未初始化")
	}
	return c.AgentManager.AddAgent(ctx, roomID)
}

func (c *LocalRuntimeController) PauseAgent(ctx context.Context, roomID int64) error {
	if c.AgentManager == nil {
		return fmt.Errorf("AgentManager 未初始化")
	}
	return c.AgentManager.PauseAgent(ctx, roomID)
}
