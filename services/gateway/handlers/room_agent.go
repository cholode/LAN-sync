package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	agentmodel "lan-im-go/services/agent/models"
	roomsmodel "lan-im-go/services/rooms/models"
)

// RoomAgentHandler 只管理当前群聊的 Agent 绑定，不删除其他群聊共用的 Bot 账号。
func RoomAgentHandler(db *gorm.DB, action string, leaveRoom ...func(int64, int64)) gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || roomID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "群号无效"})
			return
		}
		code := http.StatusInternalServerError
		var removedUserIDs []int64
		err = db.Transaction(func(tx *gorm.DB) error {
			var room roomsmodel.Room
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&room, roomID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					code = http.StatusNotFound
					return errors.New("群聊不存在")
				}
				return err
			}
			var member roomsmodel.RoomMember
			if c.GetInt8("user_role") != 1 && room.CreatorID != c.GetInt64("user_id") {
				if err := tx.Where("room_id = ? AND user_id = ?", roomID, c.GetInt64("user_id")).First(&member).Error; err != nil || member.Role < 2 {
					code = http.StatusForbidden
					return errors.New("仅群主或管理员可以管理 Agent")
				}
			}
			values := map[string]interface{}{"agent_enabled": action == "enable"}
			if tx.Migrator().HasTable("room_agent_bindings") {
				bindings := func() *gorm.DB {
					return tx.Table("room_agent_bindings").Where("room_id = ? AND deleted_at = 0", roomID)
				}
				if action == "remove" {
					var ids []int64
					if err := bindings().Pluck("id", &ids).Error; err != nil {
						return err
					}
					if err := tx.Table("room_agent_bindings").Where("room_id = ? AND deleted_at = 0 AND legacy_bot_user_id IS NOT NULL", roomID).Pluck("legacy_bot_user_id", &removedUserIDs).Error; err != nil {
						return err
					}
					stamp := time.Now().UnixMilli()
					if len(ids) > 0 {
						if err := tx.Table("room_agent_configs").Where("binding_id IN ? AND deleted_at = 0", ids).Update("deleted_at", stamp).Error; err != nil {
							return err
						}
					}
					if err := bindings().Updates(map[string]interface{}{"enabled": false, "deleted_at": stamp}).Error; err != nil {
						return err
					}
				} else {
					var count int64
					if err := bindings().Count(&count).Error; err != nil {
						return err
					}
					if action == "enable" && count == 0 {
						code = http.StatusConflict
						return errors.New("尚未绑定 Agent，请先配置群 Agent 账号")
					}
					if err := bindings().Update("enabled", action == "enable").Error; err != nil {
						return err
					}
				}
			} else if action == "enable" && room.BotUserID == 0 {
				code = http.StatusConflict
				return errors.New("尚未绑定 Agent，请先配置群 Agent 账号")
			}
			if action == "remove" {
				values["bot_user_id"] = 0
				if room.BotUserID != 0 {
					removedUserIDs = append(removedUserIDs, room.BotUserID)
				}
				if len(removedUserIDs) > 0 {
					if err := tx.Where("room_id = ? AND user_id IN ?", roomID, removedUserIDs).Delete(&roomsmodel.RoomMember{}).Error; err != nil {
						return err
					}
				}
				if tx.Migrator().HasTable(&agentmodel.AgentConfig{}) {
					if err := tx.Where("room_id = ?", roomID).Delete(&agentmodel.AgentConfig{}).Error; err != nil {
						return err
					}
				}
			}
			return tx.Model(&room).Updates(values).Error
		})
		if err != nil {
			message := err.Error()
			if code == http.StatusInternalServerError {
				message = "Agent 操作失败，请稍后重试"
			}
			c.JSON(code, gin.H{"error": message})
			return
		}
		for _, notify := range leaveRoom {
			for _, userID := range removedUserIDs {
				notify(userID, roomID)
			}
		}
		c.JSON(http.StatusOK, gin.H{"agent_enabled": action == "enable"})
	}
}
