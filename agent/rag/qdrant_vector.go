package rag

import (
	"context"
	"fmt"
	"lan-im-go/models"
	"log"
	"os"
	"strings"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// QdrantVectorStore Qdrant 向量存储实现
type QdrantVectorStore struct {
	client *qdrant.Client
}

// NewQdrantVectorStore 创建 Qdrant 向量存储
func NewQdrantVectorStore() (*QdrantVectorStore, error) {
	host := os.Getenv("QDRANT_HOST")
	if host == "" {
		host = "localhost"
	}
	port := 6334

	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant connect: %w", err)
	}

	log.Printf("[Qdrant] 已连接 %s:%d", host, port)
	return &QdrantVectorStore{client: client}, nil
}

func (s *QdrantVectorStore) collectionName(roomID int64) string {
	return fmt.Sprintf("rag_room_%d", roomID)
}

// EnsureIndex 确保集合存在，每个群独立 collection 实现向量隔离
func (s *QdrantVectorStore) EnsureIndex(ctx context.Context, roomID int64) error {
	name := s.collectionName(roomID)

	exists, err := s.client.CollectionExists(ctx, name)
	if err != nil {
		return fmt.Errorf("check collection: %w", err)
	}
	if exists {
		return nil
	}

	// 创建相应群的集合
	err = s.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     1536,
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		return fmt.Errorf("create collection: %w", err)
	}

	// chunk_type 索引，用于按类型过滤
	fTypeKeyword := qdrant.FieldType_FieldTypeKeyword
	s.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
		CollectionName: name,
		FieldName:      "chunk_type",
		FieldType:      &fTypeKeyword,
	})

	log.Printf("[Qdrant] 集合 %s 创建成功", name)
	return nil
}

// Insert 批量插入 topic 分块+向量
func (s *QdrantVectorStore) Insert(ctx context.Context, chunks []*models.RAGChunk, vectors [][]float32) error {
	if len(chunks) != len(vectors) {
		return fmt.Errorf("chunks and vectors length mismatch: %d vs %d", len(chunks), len(vectors))
	}

	name := s.collectionName(chunks[0].RoomID)

	points := make([]*qdrant.PointStruct, len(chunks))
	for i, chunk := range chunks {
		points[i] = &qdrant.PointStruct{
			Id:      qdrant.NewIDNum(uint64(chunk.ID)),
			Vectors: qdrant.NewVectors(vectors[i]...),
			Payload: qdrant.NewValueMap(map[string]any{
				"content":    chunk.Content,
				"chunk_type": chunk.ChunkType,
				"start_time": chunk.StartTime.UnixMilli(),
				"end_time":   chunk.EndTime.UnixMilli(),
				"topic_name": chunk.TopicName,
				"speakers":   chunk.Speakers,
			}),
		}
	}

	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: name,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("qdrant upsert: %w", err)
	}
	return nil
}

// Search 仅按 chunk_type 过滤的语义搜索
func (s *QdrantVectorStore) Search(ctx context.Context, queryVec []float32, roomID int64, opts SearchOptions) ([]ChunkResult, error) {
	name := s.collectionName(roomID)
	topK := uint64(opts.TopK)
	if topK == 0 {
		topK = 5
	}

	limit := topK
	req := &qdrant.QueryPoints{
		CollectionName: name,
		Query:          qdrant.NewQuery(queryVec...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
	}

	// chunk_type 过滤
	if len(opts.ChunkTypes) > 0 {
		req.Filter = &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatchKeywords("chunk_type", opts.ChunkTypes...),
			},
		}
	}

	resp, err := s.client.Query(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("qdrant query: %w", err)
	}

	return s.parseSearchResults(resp, roomID), nil
}

// 将从 Qdrant 获取的格式 payload 转化成项目能使用的格式
func (s *QdrantVectorStore) parseSearchResults(points []*qdrant.ScoredPoint, roomID int64) []ChunkResult {
	var results []ChunkResult

	for _, sp := range points {
		payload := sp.GetPayload()
		if payload == nil {
			continue
		}

		chunk := &models.RAGChunk{
			RoomID:    roomID,
			ChunkType: getString(payload, "chunk_type"),
			Content:   getString(payload, "content"),
			TopicName: getString(payload, "topic_name"),
			Speakers:  getString(payload, "speakers"),
		}

		if st := getFloat(payload, "start_time"); st > 0 {
			chunk.StartTime = time.UnixMilli(int64(st))
		}
		if et := getFloat(payload, "end_time"); et > 0 {
			chunk.EndTime = time.UnixMilli(int64(et))
		}

		if sp.Id != nil {
			if numId := sp.Id.GetNum(); numId > 0 {
				chunk.ID = int64(numId)
			}
		}

		results = append(results, ChunkResult{
			Chunk:      chunk,
			Similarity: float64(sp.GetScore()),
			Score:      float64(sp.GetScore()),
		})
	}

	return results
}

// DeleteByRoom 删除房间整个集合
func (s *QdrantVectorStore) DeleteByRoom(ctx context.Context, roomID int64) error {
	err := s.client.DeleteCollection(ctx, s.collectionName(roomID))
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("delete collection: %w", err)
	}
	return nil
}

// DeleteByDocID 按文档 ID 删除相关向量
func (s *QdrantVectorStore) DeleteByDocID(ctx context.Context, roomID, docID int64) error {
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collectionName(roomID),
		Points: qdrant.NewPointsSelectorFilter(&qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatchText("message_ids", fmt.Sprintf("%d", docID)),
			},
		}),
	})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("delete points: %w", err)
	}
	return nil
}

func getString(payload map[string]*qdrant.Value, key string) string {
	v, ok := payload[key]
	if !ok || v == nil {
		return ""
	}
	return v.GetStringValue()
}

func getFloat(payload map[string]*qdrant.Value, key string) float64 {
	v, ok := payload[key]
	if !ok || v == nil {
		return 0
	}
	if iv := v.GetIntegerValue(); iv != 0 {
		return float64(iv)
	}
	return v.GetDoubleValue()
}

var _ VectorStore = (*QdrantVectorStore)(nil)
