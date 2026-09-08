package messages

import (
	"context"
	"encoding/json"
	"errors"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"lan-im-go/repository"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIndependentMessageRoutesRequireAuthentication(t *testing.T) {
	router := NewRouter(&Module{}, func(context.Context) error { return nil })
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/v1/rooms/1/messages"}, {"GET", "/api/v1/rooms/1/messages/search"},
		{"POST", "/api/v1/files/presign"}, {"POST", "/api/v1/files/complete"},
		{"GET", "/api/v1/files/1/download"}, {"GET", "/api/v1/download/example.txt"},
	} {
		t.Run(route.path, func(t *testing.T) {
			r := httptest.NewRecorder()
			router.ServeHTTP(r, httptest.NewRequest(route.method, route.path, nil))
			if r.Code != http.StatusUnauthorized {
				t.Fatalf("接口缺少鉴权或未注册: %d", r.Code)
			}
		})
	}
}

type membershipStub struct {
	repository.RoomMemberRepository
	allowed bool
}

func (s membershipStub) CheckIsMember(roomID, userID int64) (bool, error) {
	return s.allowed && roomID == 7 && userID == 9, nil
}

type historyStub struct{ repository.MessageRepository }

func (historyStub) GetHistoryByCursor(roomID, cursor int64, limit int) ([]*models.Message, error) {
	return []*models.Message{{ID: 9007199254740993, RoomID: roomID, SenderID: 9, Content: "拆分后接口兼容"}}, nil
}

func TestIndependentHistoryPreservesMembershipAndMessageShape(t *testing.T) {
	token, err := pkg.GenerateToken(9, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, allowed := range []bool{false, true} {
		module := &Module{Repository: historyStub{}, Membership: membershipStub{allowed: allowed}}
		router := NewRouter(module, func(context.Context) error { return nil })
		req := httptest.NewRequest("GET", "/api/v1/rooms/7/messages?cursor=1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		r := httptest.NewRecorder()
		router.ServeHTTP(r, req)
		if !allowed {
			if r.Code != http.StatusForbidden {
				t.Fatalf("非群成员可读取消息: %d", r.Code)
			}
			continue
		}
		var body struct {
			Messages []struct {
				ID      string
				Content string
			}
		}
		if err := json.Unmarshal(r.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if r.Code != http.StatusOK || len(body.Messages) != 1 || body.Messages[0].ID != "9007199254740993" {
			t.Fatalf("响应格式不兼容: %d %s", r.Code, r.Body.String())
		}
	}
}

func TestReadinessDoesNotExposeDependencyError(t *testing.T) {
	router := NewRouter(&Module{}, func(context.Context) error { return errors.New("敏感连接信息") })
	r := httptest.NewRecorder()
	router.ServeHTTP(r, httptest.NewRequest("GET", "/health/ready", nil))
	if r.Code != http.StatusServiceUnavailable || r.Body.String() != `{"status":"unavailable"}` {
		t.Fatalf("就绪检查返回错误: %d %s", r.Code, r.Body.String())
	}
}
