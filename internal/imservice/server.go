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
	"lan-im-go/core"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"lan-im-go/repository"
)

// Server is the Go-side IMService that the Python agent service calls back
// into when a tool needs to read messages, kick a user, or post a reply.
type Server struct {
	agentv1.UnimplementedIMServiceServer

	hub *core.Hub
}

// NewServer creates an IMService gRPC server implementation.
func NewServer(hub *core.Hub) *Server {
	return &Server{hub: hub}
}

// Start listens on addr until ctx is cancelled.
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

// FetchMessages returns messages in [start, end) ordered ascending.
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

// KickUser removes a member from a room and forces its live connection closed.
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

// SendReply publishes a message as the bot user.
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
	select {
	case s.hub.RoomActionChan <- &core.RoomAction{UserID: userID, RoomID: roomID, Action: action}:
	default:
	}
}

func (s *Server) disconnectUser(userID int64) {
	select {
	case s.hub.Kick <- userID:
	default:
	}
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