package migration

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/yangshoulai/hydra/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// v1_0_0_Init 初始化数据库 Schema（合并所有迁移）
func v1_0_0_Init() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.0.0_init",
		Migrate: func(tx *gorm.DB) error {
			// 使用 GORM AutoMigrate 自动创建所有表
			// GORM 会根据数据库类型生成对应的 SQL
			if err := tx.AutoMigrate(
				&models.Provider{},
				&models.AdminUser{},
				&models.AccessToken{},
				&models.SystemSetting{},
				&models.Channel{},
				&models.Key{},
				&models.ChannelModelConfig{},
				&models.Model{},
				&models.RequestLog{},
			); err != nil {
				return err
			}

			// 创建额外的索引（GORM 可能不会创建所有优化的复合索引）
			if err := createAdditionalIndexes(tx); err != nil {
				return err
			}

			// 插入默认数据
			if err := seedData(tx); err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 删除所有表
			return tx.Migrator().DropTable(
				&models.RequestLog{},
				&models.Model{},
				&models.Provider{},
				&models.ChannelModelConfig{},
				&models.Key{},
				&models.Channel{},
				&models.SystemSetting{},
				&models.AccessToken{},
				&models.AdminUser{},
			)
		},
	}
}

// createAdditionalIndexes 创建额外的优化索引
func createAdditionalIndexes(db *gorm.DB) error {
	// GORM 的 AutoMigrate 会创建基本的索引，但我们需要额外的复合索引来优化查询

	// request_logs 表的复合索引
	indexes := []string{
		// 索引: trace_id + created_at (用于按 TraceID 查询和排序)
		`CREATE INDEX IF NOT EXISTS idx_request_logs_trace_id_created_at ON request_logs(trace_id, created_at DESC)`,

		// 索引: access_token + created_at (用于按 Token 查询请求历史)
		`CREATE INDEX IF NOT EXISTS idx_request_logs_access_token_created_at ON request_logs(access_token, created_at DESC)`,

		// 索引: channel_id + created_at (用于按渠道查询统计)
		`CREATE INDEX IF NOT EXISTS idx_request_logs_channel_id_created_at ON request_logs(channel_id, created_at DESC)`,

		// 索引: requested_model + created_at (用于按模型查询统计)
		`CREATE INDEX IF NOT EXISTS idx_request_logs_requested_model_created_at ON request_logs(requested_model, created_at DESC)`,

		// 索引: status_code + created_at (用于按状态码查询失败请求)
		`CREATE INDEX IF NOT EXISTS idx_request_logs_status_code_created_at ON request_logs(status_code, created_at DESC)`,

		// 索引: is_success + created_at (用于快速统计成功/失败率)
		`CREATE INDEX IF NOT EXISTS idx_request_logs_is_success_created_at ON request_logs(is_success, created_at DESC)`,

		// 索引: created_at DESC (用于时间范围查询)
		`CREATE INDEX IF NOT EXISTS idx_request_logs_created_at_desc ON request_logs(created_at DESC)`,

		// 索引: channel_id + status_code + created_at (复合索引用于渠道成功率统计)
		`CREATE INDEX IF NOT EXISTS idx_request_logs_channel_status_created_at ON request_logs(channel_id, status_code, created_at DESC)`,

		// 索引: is_stream + created_at (用于流式请求查询)
		`CREATE INDEX IF NOT EXISTS idx_request_logs_is_stream_created_at ON request_logs(is_stream, created_at DESC)`,

		// 索引: unified_model
		`CREATE INDEX IF NOT EXISTS idx_request_logs_unified_model ON request_logs(unified_model)`,
	}

	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			// 索引创建失败不应该阻止整个迁移，可能是索引已存在
			// 只记录日志，不返回错误
			db.Logger.Warn(nil, "failed to create index", "error", err.Error())
		}
	}

	return nil
}

// seedData 插入默认数据
func seedData(db *gorm.DB) error {
	// 只插入 admin_users 表的默认数据
	// 生成密码哈希 (admin123)
	hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	now := time.Now()
	adminUser := &models.AdminUser{
		Username:     "hydra",
		PasswordHash: string(hash),
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// 检查是否已存在管理员用户
	var count int64
	if err := db.Model(&models.AdminUser{}).Count(&count).Error; err != nil {
		return err
	}

	// 只有在没有管理员用户时才插入默认数据
	if count == 0 {
		if err := db.Create(adminUser).Error; err != nil {
			return err
		}
	}

	return nil
}
