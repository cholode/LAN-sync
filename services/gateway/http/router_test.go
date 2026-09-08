package gateways

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "lan-im-go/services/gateway/websocket"
)

func TestMigratedRoutesAreAbsentFromGateway(t *testing.T) {
	router := NewRouter(Dependencies{Hub: &core.Hub{}, FrontendDir: t.TempDir()})
	for _, path := range []string{"/api/v1/rooms", "/api/v1/my_rooms", "/api/v1/rooms/1/members", "/api/v1/rooms/1/messages", "/api/v1/files/1/download"} {
		r := httptest.NewRecorder()
		router.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != http.StatusNotFound || r.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Fatalf("迁出接口仍被 Gateway 处理: %s %d", path, r.Code)
		}
	}
}
