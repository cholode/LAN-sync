package chunker

import (
	"context"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"os"
	"time"

	"gorm.io/gorm"
)

// ChunkingPipeline 分块流水线，定期触发话题分块
type ChunkingPipeline struct {
	roomID       int64
	db           *gorm.DB
	topicChunker *TopicChunker
	minTopicMsgs int
	interval     time.Duration
}

// NewChunkingPipeline 创建流水线，间隔从 TOPIC_CHUNK_INTERVAL 环境变量读取，默认 5 分钟
func NewChunkingPipeline(
	roomID int64,
	db *gorm.DB,
	topicChunker *TopicChunker,
	minTopicMsgs int,
) *ChunkingPipeline {
	interval := 5 * time.Minute
	if s := os.Getenv("TOPIC_CHUNK_INTERVAL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			interval = d
		}
	}

	return &ChunkingPipeline{
		roomID:       roomID,
		db:           db,
		topicChunker: topicChunker,
		minTopicMsgs: minTopicMsgs,
		interval:     interval,
	}
}

// Start 启动定时分块
func (p *ChunkingPipeline) Start(ctx context.Context) {
	pkg.Infof("[Pipeline] room=%d 话题分块流水线启动, 间隔=%s", p.roomID, p.interval)

	topicTicker := time.NewTicker(p.interval)
	defer topicTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-topicTicker.C:
			p.tryTopicChunk(ctx)
		}
	}
}

func (p *ChunkingPipeline) tryTopicChunk(ctx context.Context) {
	count, err := p.countNewMessages(ctx, p.topicChunker.LastID())
	if err != nil {
		return
	}
	if count < int64(p.minTopicMsgs) {
		return
	}
	pkg.Infof("[Pipeline] room=%d 累积 %d 条新消息，触发话题分块", p.roomID, count)
	if err := p.topicChunker.Chunk(ctx); err != nil {
		pkg.Infof("[Pipeline] room=%d 话题分块失败: %v", p.roomID, err)
	}
}

// 计算有多少新消息
func (p *ChunkingPipeline) countNewMessages(ctx context.Context, sinceID int64) (int64, error) {
	var count int64
	err := p.db.WithContext(ctx).
		Model(&models.Message{}).
		Where("room_id = ? AND id > ?", p.roomID, sinceID).
		Count(&count).Error
	return count, err
}
