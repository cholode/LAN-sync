package rag

import (
	"fmt"
	"lan-im-go/models"
	"strings"
	"time"
)

// ContentFormatter 消息内容格式化器，确保每条消息保留用户+时间信息
type ContentFormatter struct{}

// NewContentFormatter 创建格式化器
func NewContentFormatter() *ContentFormatter {
	return &ContentFormatter{}
}

// MessageWithUser 带用户名的消息
type MessageWithUser struct {
	models.Message
	UserName string
}

// FormatTopicChunk 话题分块格式化，输出话题名、时间范围、参与者、逐条消息
func (f *ContentFormatter) FormatTopicChunk(
	topicName string,
	speakers []string,
	startTime, endTime time.Time,
	messages []MessageWithUser,
) string {
	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("话题: %s\n", topicName))
	buf.WriteString(fmt.Sprintf("时间范围: %s ~ %s\n",
		startTime.Format("2006-01-02 15:04"),
		endTime.Format("2006-01-02 15:04")))
	buf.WriteString(fmt.Sprintf("参与者: %s\n\n", strings.Join(speakers, "、")))

	for _, m := range messages {
		buf.WriteString(fmt.Sprintf("[%s] %s: %s\n",
			m.CreatedAt.Format("2006-01-02 15:04"),
			m.UserName,
			m.Content))
	}

	return buf.String()
}

// FormatMessagesForChunking 为话题切分准备消息文本（发给 LLM，每行有序号索引）
func (f *ContentFormatter) FormatMessagesForChunking(messages []MessageWithUser) string {
	var buf strings.Builder
	for i, m := range messages {
		buf.WriteString(fmt.Sprintf("[%d] [%s] %s: %s\n",
			i,
			m.CreatedAt.Format("2006-01-02 15:04"),
			m.UserName,
			m.Content))
	}
	return buf.String()
}