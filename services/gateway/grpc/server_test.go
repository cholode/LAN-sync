package imservice

import (
	"context"
	"testing"
	"time"

	agentv1 "lan-im-go/proto/agent/v1"
	messagesmodel "lan-im-go/services/messages/models"
	usersmodel "lan-im-go/services/users/models"
)

type userReaderStub struct{ name string }

func (s userReaderStub) GetByID(id int64) (*usersmodel.User, error) {
	return &usersmodel.User{Username: s.name}, nil
}

type messageReaderStub struct{ content string }

func (s messageReaderStub) GetMessagesByTimeRange(roomID int64, start, end time.Time, limit int) ([]messagesmodel.Message, error) {
	return []messagesmodel.Message{{ID: 1, RoomID: roomID, SenderID: 2, Content: s.content}}, nil
}

func TestServersUseIndependentInjectedRepositories(t *testing.T) {
	first := NewServer(nil, userReaderStub{"first-user"}, nil, messageReaderStub{"first-message"})
	second := NewServer(nil, userReaderStub{"second-user"}, nil, messageReaderStub{"second-message"})
	for _, tc := range []struct {
		server        *Server
		name, content string
	}{{first, "first-user", "first-message"}, {second, "second-user", "second-message"}, {first, "first-user", "first-message"}} {
		result, err := tc.server.FetchMessages(context.Background(), &agentv1.FetchMessagesRequest{RoomId: 7})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Messages) != 1 || result.Messages[0].SenderName != tc.name || result.Messages[0].Content != tc.content {
			t.Fatalf("server used another instance's dependencies: %+v", result)
		}
	}
}
