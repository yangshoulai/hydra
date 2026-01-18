package config

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDatabase 初始化数据库连接
func InitDatabase(cfg *Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	// 选择数据库驱动
	switch cfg.Database.Type {
	case "sqlite":
		dialector = sqlite.Open(cfg.Database.SQLitePath)
	case "postgres":
		dialector = postgres.Open(cfg.Database.PostgresDSN)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
	}

	// 配置 GORM
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // 使用自定义日志
		NowFunc: func() time.Time {
			return time.Now().UTC() // 数据库存储 UTC 时间
		},
		DisableForeignKeyConstraintWhenMigrating: false,
	}

	// 打开连接
	db, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 获取底层数据库连接
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// 配置连接池参数
	// SetMaxOpenConns: 数据库的最大打开连接数
	// - SQLite: 建议设置为 1（避免文件锁问题）
	// - PostgreSQL: 根据服务器性能和并发需求设置，建议 10-100
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)

	// SetMaxIdleConns: 数据库的最大空闲连接数
	// - 应该小于或等于 MaxOpenConns
	// - 建议设置为 MaxOpenConns 的 50% 左右
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)

	// SetConnMaxLifetime: 连接的最大存活时间
	// - 超过此时间的连接会被关闭，避免长时间使用的连接累积问题
	// - 建议设置为 5-30 分钟
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	// SetConnMaxIdleTime: 空闲连接的最大存活时间
	// - 超过此时间未使用的连接会被关闭，释放资源
	// - 建议设置为 1-5 分钟
	sqlDB.SetConnMaxIdleTime(cfg.Database.ConnMaxLifetime / 2)

	return db, nil
}

// CloseDatabase 关闭数据库连接
func CloseDatabase(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
