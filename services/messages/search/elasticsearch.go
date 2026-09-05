package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	"lan-im-go/models"
	"lan-im-go/pkg"
)

var (
	esClient  *elasticsearch.Client
	enabled   bool
	indexName = "lan-im-messages"
)

const messageIndexMapping = `{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0
  },
  "mappings": {
    "properties": {
      "id": { "type": "long" },
      "room_id": { "type": "long" },
      "sender_id": { "type": "long" },
      "client_msg_id": { "type": "keyword" },
      "type": { "type": "byte" },
      "content": { "type": "text", "analyzer": "standard" },
      "created_at": { "type": "date" },
      "deleted_at": { "type": "date" }
    }
  }
}`

// messageDoc 是聊天消息在 Elasticsearch 中的表示形式。
type messageDoc struct {
	ID          int64     `json:"id"`
	RoomID      int64     `json:"room_id"`
	SenderID    int64     `json:"sender_id"`
	ClientMsgID string    `json:"client_msg_id,omitempty"`
	Type        int8      `json:"type"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
	DeletedAt   int64     `json:"deleted_at,omitempty"`
}

// SearchParams 定义消息搜索请求的过滤条件。
type SearchParams struct {
	Query    string
	SenderID int64
	Start    time.Time
	End      time.Time
	From     int
	Size     int
}

// MessageHit 表示 Elasticsearch 返回的一条匹配消息。
type MessageHit struct {
	ID          int64     `json:"id,string"`
	RoomID      int64     `json:"room_id,string"`
	SenderID    int64     `json:"sender_id,string"`
	ClientMsgID string    `json:"client_msg_id,omitempty"`
	Type        int8      `json:"type"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
	Highlight   []string  `json:"highlight,omitempty"`
}

// SearchResult 是面向 API 的搜索响应数据。
type SearchResult struct {
	Total int64        `json:"total"`
	Hits  []MessageHit `json:"messages"`
}

// Enabled 表示 Elasticsearch 索引和搜索功能是否启用。
func Enabled() bool {
	return enabled
}

// Init 连接 Elasticsearch，并在需要时创建消息索引。
// 当 ES_ENABLED 不为 "true" 时不执行任何操作，因此系统其他部分仍可在没有 Elasticsearch 时运行。
func Init(ctx context.Context) error {
	enabled = strings.EqualFold(os.Getenv("ES_ENABLED"), "true")
	if !enabled {
		pkg.Infoln("[Elasticsearch] disabled, message search will use the primary message store")
		return nil
	}

	addr := os.Getenv("ES_ADDR")
	if addr == "" {
		addr = "http://127.0.0.1:9200"
	}
	if v := os.Getenv("ES_INDEX_MESSAGES"); v != "" {
		indexName = v
	}

	cfg := elasticsearch.Config{Addresses: []string{addr}}
	if username := os.Getenv("ES_USERNAME"); username != "" {
		cfg.Username = username
		cfg.Password = os.Getenv("ES_PASSWORD")
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	esClient = client

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := esClient.Ping(esClient.Ping.WithContext(pingCtx))
	if err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	if err := checkResponse(res); err != nil {
		return err
	}

	if err := ensureMessageIndex(ctx); err != nil {
		return err
	}

	pkg.Infof("[Elasticsearch] connected, addr=%s index=%s", addr, indexName)
	return nil
}

// Close 释放 Elasticsearch 客户端。
func Close() {
	esClient = nil
	enabled = false
}

func ensureMessageIndex(ctx context.Context) error {
	existsRes, err := esClient.Indices.Exists(
		[]string{indexName},
		esClient.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("check index existence: %w", err)
	}
	defer existsRes.Body.Close()
	if existsRes.IsError() && existsRes.StatusCode != 404 {
		body, _ := io.ReadAll(existsRes.Body)
		return fmt.Errorf("check index existence: %s", strings.TrimSpace(string(body)))
	}
	if existsRes.StatusCode == 200 {
		return nil
	}

	createRes, err := esClient.Indices.Create(
		indexName,
		esClient.Indices.Create.WithContext(ctx),
		esClient.Indices.Create.WithBody(strings.NewReader(messageIndexMapping)),
	)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	return checkResponse(createRes)
}

// IndexMessages 从调用方视角异步批量索引归档消息。
// Kafka 归档器应在消息持久化后调用此函数。
func IndexMessages(ctx context.Context, msgs []*models.Message) error {
	if !enabled || esClient == nil || len(msgs) == 0 {
		return nil
	}

	var body bytes.Buffer
	for _, msg := range msgs {
		if msg == nil {
			continue
		}

		id := msg.ClientMsgID
		if id == "" {
			id = strconv.FormatInt(msg.ID, 10)
		}

		meta, err := json.Marshal(map[string]any{
			"index": map[string]any{
				"_index": indexName,
				"_id":    id,
			},
		})
		if err != nil {
			return fmt.Errorf("encode index metadata: %w", err)
		}

		doc, err := json.Marshal(toMessageDoc(msg))
		if err != nil {
			return fmt.Errorf("encode message document: %w", err)
		}

		body.Write(meta)
		body.WriteByte('\n')
		body.Write(doc)
		body.WriteByte('\n')
	}

	res, err := esClient.Bulk(
		bytes.NewReader(body.Bytes()),
		esClient.Bulk.WithContext(ctx),
		esClient.Bulk.WithIndex(indexName),
	)
	if err != nil {
		return fmt.Errorf("bulk index: %w", err)
	}
	return checkResponse(res)
}

// SearchMessages 查询群聊消息。调用方必须事先确认当前用户属于目标群聊。
func SearchMessages(ctx context.Context, roomID int64, params SearchParams) (*SearchResult, error) {
	if !enabled || esClient == nil {
		return nil, fmt.Errorf("elasticsearch is not enabled")
	}
	if params.Size <= 0 {
		params.Size = 20
	}
	if params.Size > 100 {
		params.Size = 100
	}

	must := []any{
		map[string]any{"term": map[string]any{"room_id": roomID}},
	}
	if strings.TrimSpace(params.Query) != "" {
		must = append(must, map[string]any{
			"multi_match": map[string]any{
				"query":  params.Query,
				"fields": []string{"content"},
				"type":   "best_fields",
			},
		})
	}
	if params.SenderID > 0 {
		must = append(must, map[string]any{
			"term": map[string]any{"sender_id": params.SenderID},
		})
	}
	if !params.Start.IsZero() || !params.End.IsZero() {
		rangeQuery := map[string]any{}
		if !params.Start.IsZero() {
			rangeQuery["gte"] = params.Start
		}
		if !params.End.IsZero() {
			rangeQuery["lt"] = params.End
		}
		must = append(must, map[string]any{
			"range": map[string]any{"created_at": rangeQuery},
		})
	}

	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": must,
				"must_not": []any{
					map[string]any{"exists": map[string]any{"field": "deleted_at"}},
				},
			},
		},
		"from": params.From,
		"size": params.Size,
		"sort": []any{
			map[string]any{"created_at": map[string]any{"order": "desc"}},
			map[string]any{"id": map[string]any{"order": "desc"}},
		},
		"highlight": map[string]any{
			"fields": map[string]any{
				"content": map[string]any{
					"fragment_size":       150,
					"number_of_fragments": 1,
				},
			},
		},
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("encode search query: %w", err)
	}

	res, err := esClient.Search(
		esClient.Search.WithContext(ctx),
		esClient.Search.WithIndex(indexName),
		esClient.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("search: %s", strings.TrimSpace(string(body)))
	}

	var decoded struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source    messageDoc          `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	out := &SearchResult{
		Total: decoded.Hits.Total.Value,
		Hits:  make([]MessageHit, 0, len(decoded.Hits.Hits)),
	}
	for _, hit := range decoded.Hits.Hits {
		out.Hits = append(out.Hits, MessageHit{
			ID:          hit.Source.ID,
			RoomID:      hit.Source.RoomID,
			SenderID:    hit.Source.SenderID,
			ClientMsgID: hit.Source.ClientMsgID,
			Type:        hit.Source.Type,
			Content:     hit.Source.Content,
			CreatedAt:   hit.Source.CreatedAt,
			Highlight:   hit.Highlight["content"],
		})
	}
	return out, nil
}

func toMessageDoc(msg *models.Message) messageDoc {
	doc := messageDoc{
		ID:          msg.ID,
		RoomID:      msg.RoomID,
		SenderID:    msg.SenderID,
		ClientMsgID: msg.ClientMsgID,
		Type:        msg.Type,
		Content:     msg.Content,
		CreatedAt:   msg.CreatedAt,
	}
	if int64(msg.DeletedAt) > 0 {
		doc.DeletedAt = int64(msg.DeletedAt)
	}
	return doc
}

func checkResponse(res *esapi.Response) error {
	if res == nil {
		return fmt.Errorf("nil response")
	}
	defer res.Body.Close()
	if !res.IsError() {
		return nil
	}
	body, _ := io.ReadAll(res.Body)
	return fmt.Errorf("elasticsearch returned %s: %s", res.Status(), strings.TrimSpace(string(body)))
}
