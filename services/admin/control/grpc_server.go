package admincontrol

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	admincontrolv1 "lan-im-go/proto/admincontrol/v1"
	"lan-im-go/services/admin/application"
)

// Server 通过 gRPC 向独立的管理后台服务暴露主 IM 服务的运行时控制平面。

const controlTokenHeader = "X-Admin-Control-Token"

type Server struct {
	admincontrolv1.UnimplementedAdminControlServiceServer

	controller admin.RuntimeController
}

func NewServer(controller admin.RuntimeController) *Server {
	return &Server{controller: controller}
}

func (s *Server) Start(ctx context.Context, addr, token string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(controlTokenInterceptor(token)))
	admincontrolv1.RegisterAdminControlServiceServer(grpcServer, s)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	return grpcServer.Serve(listener)
}

func controlTokenInterceptor(token string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if token == "" {
			return handler(ctx, req)
		}
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing control metadata")
		}
		values := md.Get(controlTokenHeader)
		if len(values) == 0 || values[0] != token {
			return nil, status.Error(codes.Unauthenticated, "invalid control token")
		}
		return handler(ctx, req)
	}
}

func (s *Server) ListConnections(ctx context.Context, _ *emptypb.Empty) (*admincontrolv1.ListConnectionsResponse, error) {
	items, err := s.controller.ListConnections(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &admincontrolv1.ListConnectionsResponse{Items: connectionSnapshotsToProto(items)}, nil
}

func (s *Server) CloseConnection(ctx context.Context, req *admincontrolv1.CloseConnectionRequest) (*admincontrolv1.CloseConnectionResponse, error) {
	if err := s.controller.CloseConnection(ctx, req.GetConnectionId()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &admincontrolv1.CloseConnectionResponse{}, nil
}

func (s *Server) KickUser(ctx context.Context, req *admincontrolv1.KickUserRequest) (*admincontrolv1.KickUserResponse, error) {
	if err := s.controller.KickUser(ctx, req.GetUserId()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &admincontrolv1.KickUserResponse{}, nil
}

func (s *Server) DisbandRoom(ctx context.Context, req *admincontrolv1.DisbandRoomRequest) (*admincontrolv1.DisbandRoomResponse, error) {
	if err := s.controller.DisbandRoom(ctx, req.GetRoomId()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &admincontrolv1.DisbandRoomResponse{}, nil
}

func (s *Server) RemoveRoomMember(ctx context.Context, req *admincontrolv1.RemoveRoomMemberRequest) (*admincontrolv1.RemoveRoomMemberResponse, error) {
	if err := s.controller.RemoveRoomMember(ctx, req.GetRoomId(), req.GetUserId()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &admincontrolv1.RemoveRoomMemberResponse{}, nil
}

func (s *Server) HubStats(ctx context.Context, _ *emptypb.Empty) (*admincontrolv1.HubStatsResponse, error) {
	stats, err := s.controller.HubStats(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &admincontrolv1.HubStatsResponse{
		ClientCount: int64(stats.ClientCount),
		RoomCount:   int64(stats.RoomCount),
	}, nil
}

func (s *Server) RuntimeSnapshots(ctx context.Context, _ *emptypb.Empty) (*admincontrolv1.RuntimeSnapshotsResponse, error) {
	runtimeSnap, agentSnap, err := s.controller.RuntimeSnapshots(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &admincontrolv1.RuntimeSnapshotsResponse{
		Runtime: runtimeSnapshotToProto(runtimeSnap),
		Agent:   agentSnapshotToProto(agentSnap),
	}, nil
}
