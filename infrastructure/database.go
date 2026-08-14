package infrastructure

import (
	"os"
	"time"

	"lan-im-go/pkg"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"lan-im-go/models"
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
		pkg.Fatalf("[错误] MySQL 连接失败，请检查DSN配置: %v", err)
	}

	// 2. 配置数据库连接池参数
	sqlDB, err := DB.DB()
	if err != nil {
		pkg.Fatalf("[错误] 获取底层数据库连接失败: %v", err)
	}
	sqlDB.SetMaxIdleConns(200)
	sqlDB.SetMaxOpenConns(1000)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 3. 自动同步数据模型至数据库表结构
	pkg.Infoln("开始同步数据库表结构...")
	migrateModels := []interface{}{
		&models.User{},
		&models.Room{},
		&models.RoomMember{},
		&models.AgentConfig{},
		&models.RAGChunk{},
	}
	if os.Getenv("MESSAGE_STORE") != "mongo" {
		migrateModels = append(migrateModels, &models.Message{})
	}

	err = DB.AutoMigrate(migrateModels...)
	if err != nil {
		pkg.Fatalf("[错误] 数据库表结构同步失败: %v", err)
	}

	pkg.Infoln("MySQL 连接成功，表结构同步完成，连接池配置生效！")
}
