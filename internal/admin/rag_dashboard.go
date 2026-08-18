package admin

import (
	"context"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"gorm.io/gorm"

	"lan-im-go/internal/metrics"
	"lan-im-go/models"
)

// RAGService 提供 RAG/Qdrant 运行看板数据。
type RAGService struct {
	db     *gorm.DB
	qdrant *qdrant.Client
}

// NewRAGService 创建 RAG 看板服务。

func NewRAGService(db *gorm.DB, qdrantClients ...*qdrant.Client) *RAGService {
	var qdrantClient *qdrant.Client
	if len(qdrantClients) > 0 {
		qdrantClient = qdrantClients[0]
	}
	return &RAGService{db: db, qdrant: qdrantClient}
}

// RAGDashboard 表示 RAG/Qdrant 看板数据。
type RAGDashboard struct {
	QdrantOnline     bool      `json:"qdrant_online"`
	CollectionCount  int       `json:"collection_count"`
	VectorCount      uint64    `json:"vector_count"`
	TodayNewVectors  int64     `json:"today_new_vectors"`
	EmbeddingQueue   int64     `json:"embedding_queue"`
	EmbeddingAvgMS   float64   `json:"embedding_avg_ms"`
	QdrantQueryAvgMS float64   `json:"qdrant_query_avg_ms"`
	RAGAvgRecall     float64   `json:"rag_avg_recall"`
	TopK             int       `json:"top_k"`
	VectorDimension  int       `json:"vector_dimension"`
	LastWriteTime    time.Time `json:"last_write_time"`
	GeneratedAt      time.Time `json:"generated_at"`
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

// Dashboard 汇总 RAG 与 Qdrant 运行指标。
func (s *RAGService) Dashboard(ctx context.Context) (*RAGDashboard, error) {
	out := &RAGDashboard{
		QdrantOnline:    false,
		VectorDimension: 1024,
		GeneratedAt:     time.Now(),
	}

	if s.qdrant == nil {
		return out, nil
	}

	requestStart := time.Now()
	collections, err := s.qdrant.ListCollections(ctx)
	metrics.ObserveQdrantRequest("list_collections", requestStart, err)
	if err != nil {
		return out, nil
	}
	out.QdrantOnline = true
	out.CollectionCount = len(collections)

	for _, name := range collections {
		requestStart = time.Now()
		info, err := s.qdrant.GetCollectionInfo(ctx, name)
		metrics.ObserveQdrantRequest("get_collection_info", requestStart, err)
		if err != nil {
			continue
		}
		if info != nil && info.PointsCount != nil {
			out.VectorCount += *info.PointsCount
		}
	}

	if err := s.db.WithContext(ctx).Model(&models.RAGChunk{}).
		Where("created_at >= ?", startOfDay(time.Now())).
		Count(&out.TodayNewVectors).Error; err != nil {
		return nil, err
	}

	var last models.RAGChunk
	if err := s.db.WithContext(ctx).Order("updated_at DESC").First(&last).Error; err == nil {
		out.LastWriteTime = last.UpdatedAt
	}

	var topK float64
	if err := s.db.WithContext(ctx).Model(&models.AgentConfig{}).
		Select("COALESCE(AVG(top_k), 5)").
		Scan(&topK).Error; err == nil && topK > 0 {
		out.TopK = int(topK)
	} else {
		out.TopK = 5
	}

	out.EmbeddingQueue = 0
	return out, nil
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
