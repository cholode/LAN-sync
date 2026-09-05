package admin

import (
	"context"
	"time"

	"gorm.io/gorm"

	"lan-im-go/models"
)

// RAGService 提供 RAG 查询记录管理。
type RAGService struct {
	db *gorm.DB
}

// NewRAGService 创建 RAG 记录管理服务。
func NewRAGService(db *gorm.DB) *RAGService {
	return &RAGService{db: db}
}

// RAGQueryRecord 表示单条 RAG 查询记录。
type RAGQueryRecord struct {
	ID               int64     `json:"id"`
	RoomID           int64     `json:"room_id"`
	UserID           int64     `json:"user_id"`
	Question         string    `json:"question"`
	QueryTime        time.Time `json:"query_time"`
	RetrievedCount   int       `json:"retrieved_count"`
	SimilarityScores string    `json:"similarity_scores"`
	QueryLatencyMS   float64   `json:"query_latency_ms"`
	UsedTimeTool     bool      `json:"used_time_tool"`
	ContextSummary   string    `json:"context_summary"`
}

// ListQueries 分页查询 RAG 查询记录。
func (s *RAGService) ListQueries(ctx context.Context, page, pageSize int, roomID int64) ([]RAGQueryRecord, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.RAGQueryLog{})
	if roomID > 0 {
		query = query.Where("room_id = ?", roomID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []models.RAGQueryLog
	if err := query.Order("query_time DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	out := make([]RAGQueryRecord, 0, len(logs))
	for _, item := range logs {
		out = append(out, RAGQueryRecord{
			ID:               item.ID,
			RoomID:           item.RoomID,
			UserID:           item.UserID,
			Question:         item.Question,
			QueryTime:        item.QueryTime,
			RetrievedCount:   item.RetrievedCount,
			SimilarityScores: item.SimilarityScores,
			QueryLatencyMS:   item.QueryLatencyMS,
			UsedTimeTool:     item.UsedTimeTool,
			ContextSummary:   item.ContextSummary,
		})
	}
	return out, total, nil
}
