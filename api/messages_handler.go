package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"lan-im-go/cache"
	"lan-im-go/repository"
)

// chatHistoryMsgDTO 历史消息 API 输出：避免 DeletedAt 等内部字段；id 用 JSON 字符串防止前端 Number 精度丢失（雪花 ID）
type chatHistoryMsgDTO struct {
	ID        int64     `json:"id,string"`
	RoomID    int64     `json:"room_id,string"`
	SenderID  int64     `json:"sender_id,string"`
	Type      int8      `json:"type"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// GetChatHistory 获取群聊历史消息（游标分页）
// 路由：GET /api/v1/rooms/:id/messages?cursor=1050&limit=50
func GetChatHistory() gin.HandlerFunc {
	return func(c *gin.Context) {
		roomIDStr := c.Param("id")
		roomID, err := strconv.ParseInt(roomIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法的群聊 ID"})
			return
		}

		// 权限校验：仅群成员可查询消息
		userID := c.GetInt64("user_id")
		isMember, err := repository.RoomMember.CheckIsMember(roomID, userID)

		if err != nil || !isMember {
			c.JSON(http.StatusForbidden, gin.H{"error": "您不是该群成员，无权查看聊天记录"})
			return
		}

		cursorStr := c.DefaultQuery("cursor", "0")
		limitStr := c.DefaultQuery("limit", "50")
		cursorMsgID, _ := strconv.ParseInt(cursorStr, 10, 64)
		limit, _ := strconv.Atoi(limitStr)

		if limit > 100 {
			limit = 100
		} else if limit <= 0 {
			limit = 50
		}

		// ====================================================================
		// 首页查询（cursor=0）：优先走 Redis 热点缓存，miss 回退 MySQL
		// ====================================================================
		var messages []cache.CachedMsg
		var nextCursor int64 = 0
		hasMore := false

		if cursorMsgID == 0 {
			cached, err := cache.GetLatestMessages(c.Request.Context(), roomID, limit)
			if err == nil && len(cached) > 0 {
				messages = cached
				// 缓存命中：判断是否还有更多（缓存满则说明 MySQL 可能还有更老的）
				hasMore = len(cached) == limit
				if hasMore && len(cached) > 0 {
					nextCursor = cached[len(cached)-1].ID
				}
				// 转 DTO 返回
				out := make([]chatHistoryMsgDTO, 0, len(messages))
				for _, m := range messages {
					out = append(out, chatHistoryMsgDTO{
						ID: m.ID, RoomID: m.RoomID, SenderID: m.SenderID,
						Type: m.Type, Content: m.Content, CreatedAt: m.CreatedAt,
					})
				}
				c.JSON(http.StatusOK, gin.H{
					"messages":    out,
					"next_cursor": strconv.FormatInt(nextCursor, 10),
					"has_more":    hasMore,
					"source":      "redis",
				})
				return
			}
		}

		// ====================================================================
		// 缓存未命中 或 翻页查询 → MySQL
		// ====================================================================
		dbMsgs, err := repository.Message.GetHistoryByCursor(roomID, cursorMsgID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取历史消息失败"})
			return
		}

		// 缓存未命中时，异步回填 Redis
		if cursorMsgID == 0 && len(dbMsgs) > 0 {
			go cache.BackfillRoomCache(c.Request.Context(), dbMsgs)
		}

		if len(dbMsgs) > 0 {
			nextCursor = dbMsgs[len(dbMsgs)-1].ID
		}
		hasMore = len(dbMsgs) == limit

		out := make([]chatHistoryMsgDTO, 0, len(dbMsgs))
		for _, m := range dbMsgs {
			out = append(out, chatHistoryMsgDTO{
				ID: m.ID, RoomID: m.RoomID, SenderID: m.SenderID,
				Type: m.Type, Content: m.Content, CreatedAt: m.CreatedAt,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"messages":    out,
			"next_cursor": strconv.FormatInt(nextCursor, 10),
			"has_more":    hasMore,
		})
	}
}
