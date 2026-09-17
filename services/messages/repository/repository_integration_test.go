package repository

import (
	"os"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	messagesmodel "lan-im-go/services/messages/models"
)

// 此测试只能连接独立测试数据库，不会自动使用业务数据库配置。
func TestMySQLArchiveReplayRetainsCanonicalMessage(t *testing.T) {
	dsn := os.Getenv("MESSAGE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未提供独立测试数据库 MESSAGE_TEST_MYSQL_DSN")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&messagesmodel.Message{}); err != nil {
		t.Fatal(err)
	}
	repo := NewMySQLRepository(db)
	key := "archive-replay-" + time.Now().Format("20060102150405.000000000")
	first := &messagesmodel.Message{ID: time.Now().UnixNano() / 1000, RoomID: 1, SenderID: 2, ClientMsgID: key, Content: "原始内容", Type: 1, CreatedAt: time.Now()}
	t.Cleanup(func() {
		if err := db.Unscoped().Where("client_msg_id = ?", key).Delete(&messagesmodel.Message{}).Error; err != nil {
			t.Errorf("清理测试消息失败: %v", err)
		}
	})
	if err := repo.SaveMessageBatch([]*messagesmodel.Message{first}); err != nil {
		t.Fatal(err)
	}
	replay := *first
	replay.ID++
	replay.Content = "重放不应改写内容"
	if err := repo.SaveMessageBatch([]*messagesmodel.Message{&replay}); err != nil {
		t.Fatal(err)
	}
	if replay.ID != first.ID || replay.Content != first.Content {
		t.Fatalf("重放产生不同消息: %+v", replay)
	}
	var count int64
	if err := db.Model(&messagesmodel.Message{}).Where("client_msg_id = ?", key).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("重复落库: count=%d err=%v", count, err)
	}
}
