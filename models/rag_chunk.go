package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// ChunkType 分块类型
type ChunkType string

const (
	ChunkTypeTopic ChunkType = "topic" // LLM 话题分块
	ChunkTypeTime  ChunkType = "time"  // 规则时间分块
)

// TimeGranularity 时间块粒度
type TimeGranularity string

const (
	GranularityHourly TimeGranularity = "hourly"
	GranularityDaily  TimeGranularity = "daily"
)

// RAGChunk RAG 分块模型
// 统一承载 topic 和 time 两种分块类型，通过 ChunkType 字段区分
type RAGChunk struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	RoomID    int64  `gorm:"type:bigint;not null;index:idx_room_type,priority=1;index:idx_room_time,priority=1;comment:'所属群ID'"`
	ChunkType string `gorm:"type:varchar(16);not null;index:idx_room_type,priority=2;comment:'分块类型: topic/time'"`

	// --- Topic 专属字段 ---
	TopicName string `gorm:"type:varchar(256);comment:'LLM 生成的话题名称'"`
	Speakers  string `gorm:"type:json;comment:'参与者列表 JSON'"`

	// --- Time 专属字段 ---
	Date        string `gorm:"type:varchar(16);comment:'日期: 2025-07-24'"`
	TimeRange   string `gorm:"type:varchar(32);comment:'时间区间: 14:00~15:00 或 全天'"`
	Granularity string `gorm:"type:varchar(16);comment:'时间粒度: hourly/daily'"`

	// --- 共用字段 ---
	Content    string                `gorm:"type:mediumtext;not null;comment:'结构化消息文本，保留用户+时间'"`
	StartTime  time.Time             `gorm:"type:datetime(3);not null;index:idx_room_time,priority=2;comment:'起始时间'"`
	EndTime    time.Time             `gorm:"type:datetime(3);not null;comment:'结束时间'"`
	MessageIDs string                `gorm:"type:json;comment:'消息ID列表 JSON'"`
	VectorID   string                `gorm:"type:varchar(128);comment:'向量存储中的唯一标识'"`
	TokenCount int                   `gorm:"type:int;default:0;comment:'content 的 token 数量'"`
	CreatedAt  time.Time             `gorm:"autoCreateTime"`
	UpdatedAt  time.Time             `gorm:"autoUpdateTime"`
	DeletedAt  soft_delete.DeletedAt `gorm:"type:bigint unsigned;index;softDelete:milli"`
}

// TableName 指定表名
func (RAGChunk) TableName() string {
	return "rag_chunks"
}