package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"lan-im-go/internal/search"
	"lan-im-go/repository"
)

// SearchMessages 通过 Elasticsearch 搜索已归档的聊天室消息。
// 路由: GET /api/v1/rooms/:id/messages/search?q=keyword&from=0&size=20
func SearchMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !search.Enabled() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Elasticsearch search is not enabled"})
			return
		}

		roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room id"})
			return
		}

		userID := c.GetInt64("user_id")
		isMember, err := repository.RoomMember.CheckIsMember(roomID, userID)
		if err != nil || !isMember {
			c.JSON(http.StatusForbidden, gin.H{"error": "room member access required"})
			return
		}

		keyword := strings.TrimSpace(c.Query("q"))
		if keyword == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
			return
		}

		params := search.SearchParams{Query: keyword}
		if v := c.Query("sender_id"); v != "" {
			params.SenderID, _ = strconv.ParseInt(v, 10, 64)
		}
		if v := c.Query("start"); v != "" {
			if ms, parseErr := strconv.ParseInt(v, 10, 64); parseErr == nil {
				params.Start = time.UnixMilli(ms)
			}
		}
		if v := c.Query("end"); v != "" {
			if ms, parseErr := strconv.ParseInt(v, 10, 64); parseErr == nil {
				params.End = time.UnixMilli(ms)
			}
		}

		params.From, _ = strconv.Atoi(c.DefaultQuery("from", "0"))
		params.Size, _ = strconv.Atoi(c.DefaultQuery("size", "20"))
		if params.From < 0 {
			params.From = 0
		}
		if params.Size > 100 {
			params.Size = 100
		}
		if params.Size <= 0 {
			params.Size = 20
		}

		result, err := search.SearchMessages(c.Request.Context(), roomID, params)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
			return
		}

		c.JSON(http.StatusOK, result)
	}
}
