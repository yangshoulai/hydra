package migration

import (
	"fmt"

	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
	"github.com/go-gormigrate/gormigrate/v2"
)

// RunMigrations 执行所有数据库迁移
func RunMigrations(db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		// v1.0.0 初始化 Schema
		v1_0_0_Init(),
		// v1.0.1 优化 RequestLog 表索引
		v1_0_1_optimize_indexes(),
		// v1.0.2 更新 access_tokens 表
		v1_0_2_access_token_updates(),
		// v1.0.3 创建统一模型表
		v1_0_3_create_models_table(),
		// v1.0.4 移除 models 表的 status 字段
		v1_0_4_remove_model_status(),
		// v1.0.5 移除 models 表的 deleted_at 字段
		v1_0_5_remove_model_deleted_at(),
		// v1.0.6 添加 access_tokens 表的 expires_at 字段
		V1_0_6_AddAccessTokenExpiresAt(),
		// v1.0.7 创建厂商表
		v1_0_7_create_providers_table(),
		// v1.0.8 修改 models 表，添加 provider 外键
		v1_0_8_modify_models_add_provider_foreign_key(),
		// v1.0.9 删除 models 表的旧 provider 列
		v1_0_9_modify_models_drop_old_provider_column(),
		// v1.1.0 重建 providers 表，使用字符串 ID
		v1_1_0_rebuild_providers_as_string(),
		// v1.1.1 修改 models 表的 provider_id 列类型
		v1_1_1_modify_models_provider_id_string(),
		// v1.1.2 更新 token_preview 格式
		v1_1_2_update_token_and_key_preview_format(),
		// v1.1.3 添加 access_tokens 表的 name 字段唯一索引
		v1_1_3_add_token_name_unique_index(),
		// v1.1.4 添加 providers 表的 lobeIcon 字段
		v1_1_4_add_providers_lobe_icon(),
	})

	if err := m.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// v1_0_0_Init 初始化数据库 Schema
func v1_0_0_Init() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.0.0_init",
		Migrate: func(tx *gorm.DB) error {
			// 创建所有表
			err := tx.AutoMigrate(
				&models.Channel{},
				&models.Key{},
				&models.ChannelModelConfig{},
				&models.RequestLog{},
				&models.SystemSetting{},
				&models.AccessToken{},
				&models.AdminUser{},
			)
			if err != nil {
				return err
			}

			// 创建默认管理员用户 (admin / admin123)
			adminUser := &models.AdminUser{
				Username: "admin",
				Status:   "active",
			}
			if err := adminUser.SetPassword("admin123"); err != nil {
				return err
			}
			if err := tx.Create(adminUser).Error; err != nil {
				return err
			}

			// 初始化系统设置
			settings := []models.SystemSetting{
				{
					Key:       models.SettingCircuitBreakerFailureThreshold,
					Value:     "3",
					ValueType: "int",
					Category:  "circuit_breaker",
					Remark:    "熔断失败阈值(连续失败次数)",
				},
				{
					Key:       models.SettingCircuitBreakerCoolingDuration,
					Value:     "60",
					ValueType: "int",
					Category:  "circuit_breaker",
					Remark:    "冷却时长(秒)",
				},
				{
					Key:       models.SettingCircuitBreakerMaxRetry,
					Value:     "3",
					ValueType: "int",
					Category:  "circuit_breaker",
					Remark:    "单个请求最大重试次数",
				},
				{
					Key:       models.SettingLogRetentionDays,
					Value:     "30",
					ValueType: "int",
					Category:  "logging",
					Remark:    "审计日志保留天数",
				},
				{
					Key:       models.SettingLogDebugEnabled,
					Value:     "false",
					ValueType: "bool",
					Category:  "logging",
					Remark:    "是否启用调试日志(记录完整 Request/Response Body)",
				},
				{
					Key:       models.SettingProxyRequestTimeout,
					Value:     "60",
					ValueType: "int",
					Category:  "proxy",
					Remark:    "代理请求超时(秒)",
				},
				{
					Key:       models.SettingProxyMaxResponseSize,
					Value:     "10485760",
					ValueType: "int",
					Category:  "proxy",
					Remark:    "最大响应 Body 大小(字节,默认 10MB)",
				},
				{
					Key:       models.SettingProxyMaxConcurrent,
					Value:     "1000",
					ValueType: "int",
					Category:  "proxy",
					Remark:    "最大并发请求数",
				},
			}

			for _, setting := range settings {
				if err := tx.Create(&setting).Error; err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 删除所有表
			return tx.Migrator().DropTable(
				&models.AdminUser{},
				&models.AccessToken{},
				&models.SystemSetting{},
				&models.RequestLog{},
				&models.ChannelModelConfig{},
				&models.Key{},
				&models.Channel{},
			)
		},
	}
}
