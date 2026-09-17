package models

import "time"

// RAGQueryLog 记录 RAG 检索查询的日志信息，用于超级管理员后台排查分析。
type RAGQueryLog struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RoomID           int64     `gorm:"type:bigint;index:idx_rag_query_room;not null" json:"room_id"`
	UserID           int64     `gorm:"type:bigint;index:idx_rag_query_user;not null" json:"user_id"`
	Question         string    `gorm:"type:text" json:"question"`
	QueryTime        time.Time `gorm:"index:idx_rag_query_time" json:"query_time"`
	RetrievedCount   int       `gorm:"type:int;default:0" json:"retrieved_count"`
	SimilarityScores string    `gorm:"type:text" json:"similarity_scores"`
	QueryLatencyMS   float64   `gorm:"type:double;default:0" json:"query_latency_ms"`
	UsedTimeTool     bool      `gorm:"type:tinyint(1);default:0" json:"used_time_tool"`
	ContextSummary   string    `gorm:"type:mediumtext" json:"context_summary"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 返回数据库表名
func (RAGQueryLog) TableName() string {
	return "rag_query_logs"
}
