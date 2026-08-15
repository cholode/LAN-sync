package admincontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"lan-im-go/core"
	"lan-im-go/internal/metrics"
)

// HTTPClient 供独立管理端服务调用主 IM 服务控制面。
type HTTPClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTPClient(baseURL, token string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		token:   token,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *HTTPClient) post(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set(controlTokenHeader, c.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("控制面请求失败: %s %s", resp.Status, bytes.TrimSpace(body))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *HTTPClient) get(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set(controlTokenHeader, c.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("控制面请求失败: %s %s", resp.Status, bytes.TrimSpace(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *HTTPClient) ListConnections(ctx context.Context) ([]core.ConnectionSnapshot, error) {
	var out []core.ConnectionSnapshot
	err := c.get(ctx, "/internal/admin/connections", &out)
	return out, err
}

func (c *HTTPClient) CloseConnection(ctx context.Context, connectionID string) error {
	return c.post(ctx, "/internal/admin/connections/"+connectionID+"/close", nil)
}

func (c *HTTPClient) KickUser(ctx context.Context, userID int64) error {
	return c.post(ctx, fmt.Sprintf("/internal/admin/users/%d/kick", userID), nil)
}

func (c *HTTPClient) DisbandRoom(ctx context.Context, roomID int64) error {
	return c.post(ctx, fmt.Sprintf("/internal/admin/rooms/%d/disband", roomID), nil)
}

func (c *HTTPClient) RemoveRoomMember(ctx context.Context, roomID, userID int64) error {
	return c.post(ctx, fmt.Sprintf("/internal/admin/rooms/%d/members/%d/remove", roomID, userID), nil)
}

func (c *HTTPClient) HubStats(ctx context.Context) (core.HubStats, error) {
	var out core.HubStats
	err := c.get(ctx, "/internal/admin/hub-stats", &out)
	return out, err
}

func (c *HTTPClient) RuntimeSnapshots(ctx context.Context) (metrics.RuntimeSnapshot, metrics.AgentRuntimeSnapshot, error) {
	var out runtimeBundle
	err := c.get(ctx, "/internal/admin/runtime", &out)
	return out.Runtime, out.Agent, err
}

func (c *HTTPClient) AddAgent(ctx context.Context, roomID int64) error {
	return c.post(ctx, fmt.Sprintf("/internal/admin/agents/%d/add", roomID), nil)
}

func (c *HTTPClient) PauseAgent(ctx context.Context, roomID int64) error {
	return c.post(ctx, fmt.Sprintf("/internal/admin/agents/%d/pause", roomID), nil)
}
