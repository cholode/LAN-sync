package infrastructure

import (
	"os"
	"time"

	adminmodel "lan-im-go/services/admin/models"
	agentmodel "lan-im-go/services/agent/models"
	messagesmodel "lan-im-go/services/messages/models"
	roomsmodel "lan-im-go/services/rooms/models"
	usersmodel "lan-im-go/services/users/models"
	"lan-im-go/shared/observability/logger"
	"lan-im-go/shared/observability/metrics"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DB 全局数据库实例，应用全局复用
var DB *gorm.DB

// InitDatabase 初始化数据库引擎，自动同步表结构
func InitDatabase(dsn string) {
	var err error
	// 1. 创建数据库连接
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		// 生产环境可关闭SQL日志输出
		// Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		logger.Fatalf("[错误] MySQL 连接失败，请检查DSN配置: %v", err)
	}

	// 2. 配置数据库连接池参数
	sqlDB, err := DB.DB()
	if err != nil {
		logger.Fatalf("[错误] 获取底层数据库连接失败: %v", err)
	}
	sqlDB.SetMaxIdleConns(200)
	sqlDB.SetMaxOpenConns(1000)
	sqlDB.SetConnMaxLifetime(time.Hour)
	metrics.RegisterMySQLPoolMetrics(sqlDB)

	// 3. 自动同步数据模型至数据库表结构
	logger.Infoln("开始同步数据库表结构...")
	migrateModels := []interface{}{
		&usersmodel.User{},
		&roomsmodel.Room{},
		&roomsmodel.RoomMember{},
		&agentmodel.AgentConfig{},
		&agentmodel.RAGChunk{},
		&adminmodel.AdminAuditLog{},
		&agentmodel.RAGQueryLog{},
		&adminmodel.ModerationEvent{},
		&messagesmodel.FileRecord{},
		&agentmodel.GlobalAgentConfig{},
		&agentmodel.AgentConfigHistory{},
		&agentmodel.ToolCallLog{},
		&adminmodel.SystemErrorLog{},
		&adminmodel.AlertEvent{},
	}
	if os.Getenv("MESSAGE_STORE") != "mongo" {
		migrateModels = append(migrateModels, &messagesmodel.Message{})
	}

	err = DB.AutoMigrate(migrateModels...)
	if err != nil {
		logger.Fatalf("[错误] 数据库表结构同步失败: %v", err)
	}
	// 新联合唯一索引创建后再移除旧全局索引，允许不同用户复用客户端凭证。
	if os.Getenv("MESSAGE_STORE") != "mongo" && DB.Migrator().HasIndex(&messagesmodel.Message{}, "idx_client_msg_id") {
		if err := DB.Migrator().DropIndex(&messagesmodel.Message{}, "idx_client_msg_id"); err != nil {
			logger.Fatalf("消息幂等索引迁移失败: %v", err)
		}
	}

	metrics.RegisterGORMMetrics(DB, "mysql")

	logger.Infoln("MySQL 连接成功，表结构同步完成，连接池配置生效！")
}
