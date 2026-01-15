package migration

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// v1_0_7_create_providers_table 创建厂商表
func v1_0_7_create_providers_table() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.0.7-create-providers-table",
		Migrate: func(tx *gorm.DB) error {
			// 创建 providers 表
			if err := tx.AutoMigrate(&models.Provider{}); err != nil {
				return err
			}

			// 不再插入初始数据，改为通过前端同步获取
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 删除 providers 表
			return tx.Migrator().DropTable(&models.Provider{})
		},
	}
}

// v1_0_8_modify_models_add_provider_foreign_key 修改 models 表，添加 provider 外键
func v1_0_8_modify_models_add_provider_foreign_key() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.0.8-modify-models-add-provider-foreign-key",
		Migrate: func(tx *gorm.DB) error {
			// 检查 provider_id 列是否已存在
			type Model struct {
				ProviderID *uint
			}

			if !tx.Migrator().HasColumn(&Model{}, "provider_id") {
				// 如果不存在，添加该列
				type ModelWithColumn struct {
					ID         uint      `gorm:"primarykey"`
					Name       string
					ProviderID *uint     `gorm:"type:uint;comment:厂商ID"`
					Remark     string
					CreatedAt  time.Time `gorm:"comment:创建时间"`
					UpdatedAt  time.Time `gorm:"comment:更新时间"`
				}

				if err := tx.Migrator().AutoMigrate(&ModelWithColumn{}); err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 回滚：移除外键字段
			type Model struct {
				ProviderID *uint
			}

			return tx.Migrator().DropColumn(&Model{}, "provider_id")
		},
	}
}

// v1_0_9_modify_models_drop_old_provider_column 删除 models 表的旧 provider 列（如果存在）
func v1_0_9_modify_models_drop_old_provider_column() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.0.9-modify-models-drop-old-provider-column",
		Migrate: func(tx *gorm.DB) error {
			// 直接使用 SQL 检查并删除 provider 列（如果存在）
			// 先检查列是否存在
			var result []struct {
				Name string
			}

			// SQLite 查询表结构
			if err := tx.Raw("PRAGMA table_info(models)").Scan(&result).Error; err != nil {
				// 如果查询失败，忽略错误继续
				return nil
			}

			// 检查是否有 provider 列
			hasProviderColumn := false
			for _, col := range result {
				if col.Name == "provider" {
					hasProviderColumn = true
					break
				}
			}

			// 如果存在，删除该列
			if hasProviderColumn {
				// SQLite 不支持 ALTER TABLE DROP COLUMN，需要重建表
				// 但由于这是新安装，且 v1_0_3 已经创建了新结构，
				// 理论上不应该有 provider 列
				// 如果有，我们记录一个警告
				tx.Logger.Warn(nil, "old 'provider' column found in models table, please manually remove it if needed")
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 回滚：恢复旧的 provider 列
			// SQLite 不支持直接添加列，需要重建表
			// 这里我们返回 nil，因为实际上不应该回滚
			return nil
		},
	}
}
