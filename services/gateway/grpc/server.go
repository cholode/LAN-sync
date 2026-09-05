package imservice

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc"

	agentv1 "lan-im-go/proto/agent/v1"

	"lan-im-go/config"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"lan-im-go/repository"
	"lan-im-go/services/gateway/websocket"
)

// Server 是 Go 侧的 IMService，供 Python Agent 工具回调，
// 用于读取消息、移除用户或发送回复。
type Server struct {
	agentv1.UnimplementedIMServiceServer

	hub *core.Hub
}

// NewServer 创建 IMService 的 gRPC 服务端实现。
func NewServer(hub *core.Hub) *Server {
	return &Server{hub: hub}
}

// Start 在 addr 上监听，直至 ctx 被取消。
func (s *Server) Start(ctx context.Context, addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()
	agentv1.RegisterIMServiceServer(grpcServer, s)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	pkg.Infof("[IMService] gRPC 服务监听 %s", addr)
	return grpcServer.Serve(lis)
}

// FetchMessages 按升序返回 [start, end) 时间范围内的消息。
func (s *Server) FetchMessages(ctx context.Context, req *agentv1.FetchMessagesRequest) (*agentv1.FetchMessagesResponse, error) {
	limit := int(req.GetLimit())
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	start := time.UnixMilli(req.GetStartTimeUnixMs())
	end := time.UnixMilli(req.GetEndTimeUnixMs())
	if end.Before(start) {
		return &agentv1.FetchMessagesResponse{Messages: []*agentv1.MessageRecord{}}, nil
	}

	msgs, err := repository.Message.GetMessagesByTimeRange(req.GetRoomId(), start, end, limit)
	if err != nil {
		return nil, err
	}

	names := resolveUserNames(msgs)
	out := make([]*agentv1.MessageRecord, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, &agentv1.MessageRecord{
			MessageId:       m.ID,
			SenderId:        m.SenderID,
			SenderName:      names[m.SenderID],
			Content:         m.Content,
			CreatedAtUnixMs: m.CreatedAt.UnixMilli(),
		})
	}

	return &agentv1.FetchMessagesResponse{Messages: out}, nil
}

// KickUser 将成员移出群聊并强制关闭其实时连接。
func (s *Server) KickUser(ctx context.Context, req *agentv1.KickUserRequest) (*agentv1.KickUserResponse, error) {
	if err := repository.RoomMember.RemoveMember(req.GetRoomId(), req.GetUserId()); err != nil {
		return &agentv1.KickUserResponse{
			Removed: false,
			Message: fmt.Sprintf("remove member failed: %v", err),
		}, nil
	}

	s.notifyRoomAction(req.GetRoomId(), req.GetUserId(), "leave")
	s.disconnectUser(req.GetUserId())

	return &agentv1.KickUserResponse{
		Removed: true,
		Message: "success",
	}, nil
}

// SendReply 以机器人用户身份发布消息。
func (s *Server) SendReply(ctx context.Context, req *agentv1.SendReplyRequest) (*agentv1.SendReplyResponse, error) {
	messageID := req.GetMessageId()
	if messageID == "" {
		messageID = fmt.Sprintf("agent-%d-%d", req.GetRoomId(), time.Now().UnixNano())
	}

	err := config.KafkaProducer.HandleIncomingMessage(
		ctx,
		strconv.FormatInt(req.GetRoomId(), 10),
		int(req.GetBotUserId()),
		req.GetContent(),
		messageID,
	)
	if err != nil {
		return nil, err
	}

	return &agentv1.SendReplyResponse{}, nil
}

func (s *Server) notifyRoomAction(roomID, userID int64, action string) {
	switch action {
	case "join":
		s.hub.JoinRoom(userID, roomID)
	case "leave":
		s.hub.LeaveRoom(userID, roomID)
	case "disband":
		s.hub.DisbandRoom(roomID)
	}
}

func (s *Server) disconnectUser(userID int64) {
	s.hub.Kick(userID)
}

func resolveUserNames(msgs []models.Message) map[int64]string {
	names := make(map[int64]string, len(msgs))
	seen := make(map[int64]struct{}, len(msgs))

	for _, m := range msgs {
		if _, ok := seen[m.SenderID]; ok {
			continue
		}
		seen[m.SenderID] = struct{}{}

		name := fmt.Sprintf("用户%d", m.SenderID)
		if user, err := repository.User.GetByID(m.SenderID); err == nil && user != nil && user.Username != "" {
			name = user.Username
		}
		names[m.SenderID] = name
	}

	return names
}
