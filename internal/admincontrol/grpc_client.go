package admincontrol

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	"lan-im-go/core"
	"lan-im-go/internal/metrics"
	admincontrolv1 "lan-im-go/proto/admincontrol/v1"
)

// GRPCClient 是独立 admin 服务访问主 IM 服务控制面的 protobuf 客户端。
type GRPCClient struct {
	conn   *grpc.ClientConn
	client admincontrolv1.AdminControlServiceClient
	token  string
}

func NewGRPCClient(addr, token string) (*GRPCClient, error) {
	if addr == "" {
		return nil, fmt.Errorf("admin control grpc addr is empty")
	}
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("create admin control grpc client: %w", err)
	}
	return &GRPCClient{
		conn:   conn,
		client: admincontrolv1.NewAdminControlServiceClient(conn),
		token:  token,
	}, nil
}

func (c *GRPCClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *GRPCClient) withToken(ctx context.Context) context.Context {
	if c.token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, controlTokenHeader, c.token)
}

func (c *GRPCClient) ListConnections(ctx context.Context) ([]core.ConnectionSnapshot, error) {
	resp, err := c.client.ListConnections(c.withToken(ctx), &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	return protoSnapshotsToConnections(resp.GetItems()), nil
}

func (c *GRPCClient) CloseConnection(ctx context.Context, connectionID string) error {
	_, err := c.client.CloseConnection(c.withToken(ctx), &admincontrolv1.CloseConnectionRequest{ConnectionId: connectionID})
	return err
}

func (c *GRPCClient) KickUser(ctx context.Context, userID int64) error {
	_, err := c.client.KickUser(c.withToken(ctx), &admincontrolv1.KickUserRequest{UserId: userID})
	return err
}

func (c *GRPCClient) DisbandRoom(ctx context.Context, roomID int64) error {
	_, err := c.client.DisbandRoom(c.withToken(ctx), &admincontrolv1.DisbandRoomRequest{RoomId: roomID})
	return err
}

func (c *GRPCClient) RemoveRoomMember(ctx context.Context, roomID, userID int64) error {
	_, err := c.client.RemoveRoomMember(c.withToken(ctx), &admincontrolv1.RemoveRoomMemberRequest{RoomId: roomID, UserId: userID})
	return err
}

func (c *GRPCClient) HubStats(ctx context.Context) (core.HubStats, error) {
	resp, err := c.client.HubStats(c.withToken(ctx), &emptypb.Empty{})
	if err != nil {
		return core.HubStats{}, err
	}
	return core.HubStats{ClientCount: int(resp.GetClientCount()), RoomCount: int(resp.GetRoomCount())}, nil
}

func (c *GRPCClient) RuntimeSnapshots(ctx context.Context) (metrics.RuntimeSnapshot, metrics.AgentRuntimeSnapshot, error) {
	resp, err := c.client.RuntimeSnapshots(c.withToken(ctx), &emptypb.Empty{})
	if err != nil {
		return metrics.RuntimeSnapshot{}, metrics.AgentRuntimeSnapshot{}, err
	}
	return protoToRuntimeSnapshot(resp.GetRuntime()), protoToAgentSnapshot(resp.GetAgent()), nil
}

func (c *GRPCClient) AddAgent(ctx context.Context, roomID int64) error {
	_, err := c.client.AddAgent(c.withToken(ctx), &admincontrolv1.AddAgentRequest{RoomId: roomID})
	return err
}

func (c *GRPCClient) PauseAgent(ctx context.Context, roomID int64) error {
	_, err := c.client.PauseAgent(c.withToken(ctx), &admincontrolv1.PauseAgentRequest{RoomId: roomID})
	return err
}
