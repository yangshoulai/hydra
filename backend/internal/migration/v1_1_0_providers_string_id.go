package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// v1_1_0_rebuild_providers_as_string 重建 providers 表，使用字符串 ID
func v1_1_0_rebuild_providers_as_string() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.1.0-rebuild-providers-as-string",
		Migrate: func(tx *gorm.DB) error {
			// 删除旧的 providers 表
			if err := tx.Migrator().DropTable(&models.Provider{}); err != nil {
				return err
			}

			// 创建新的 providers 表（使用字符串 ID）
			if err := tx.AutoMigrate(&models.Provider{}); err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 回滚：删除表
			return tx.Migrator().DropTable(&models.Provider{})
		},
	}
}

// v1_1_1_modify_models_provider_id_string 修改 models 表的 provider_id 列类型
func v1_1_1_modify_models_provider_id_string() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.1.1-modify-models-provider-id-string",
		Migrate: func(tx *gorm.DB) error {
			// SQLite 不支持 ALTER COLUMN，需要重建表
			if tx.Dialector.Name() == "sqlite" {
				// 清理可能存在的临时表
				tx.Exec(`DROP TABLE IF EXISTS models_new`)

				// 创建新表结构
				if err := tx.Exec(`
					CREATE TABLE models_new (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						name VARCHAR(100) NOT NULL UNIQUE,
						provider_id VARCHAR(50),
						remark VARCHAR(500),
						created_at DATETIME,
						updated_at DATETIME
					)
				`).Error; err != nil {
					return err
				}

				// 复制数据（将旧的 provider_id 转换为字符串）
				if err := tx.Exec(`
					INSERT INTO models_new (id, name, provider_id, remark, created_at, updated_at)
					SELECT
						id,
						name,
						CAST(provider_id AS TEXT),
						remark,
						created_at,
						updated_at
					FROM models
				`).Error; err != nil {
					return err
				}

				// 删除旧表
				if err := tx.Exec(`DROP TABLE models`).Error; err != nil {
					return err
				}

				// 重命名新表
				if err := tx.Exec(`ALTER TABLE models_new RENAME TO models`).Error; err != nil {
					return err
				}

				// 重建唯一索引
				if err := tx.Exec(`CREATE UNIQUE INDEX idx_models_name ON models(name)`).Error; err != nil {
					return err
				}
			} else {
				// MySQL/PostgreSQL: 直接修改列类型
				if err := tx.Exec(`ALTER TABLE models MODIFY COLUMN provider_id VARCHAR(50)`).Error; err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// SQLite 回滚
			if tx.Dialector.Name() == "sqlite" {
				// 清理可能存在的临时表
				tx.Exec(`DROP TABLE IF EXISTS models_old`)

				// 创建旧表结构
				if err := tx.Exec(`
					CREATE TABLE models_old (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						name VARCHAR(100) NOT NULL UNIQUE,
						provider_id INTEGER,
						remark VARCHAR(500),
						created_at DATETIME,
						updated_at DATETIME
					)
				`).Error; err != nil {
					return err
				}

				// 复制数据
				if err := tx.Exec(`
					INSERT INTO models_old (id, name, provider_id, remark, created_at, updated_at)
					SELECT id, name, CAST(provider_id AS INTEGER), remark, created_at, updated_at
					FROM models
					WHERE provider_id IS NOT NULL
				`).Error; err != nil {
					return err
				}

				// 删除新表
				if err := tx.Exec(`DROP TABLE models`).Error; err != nil {
					return err
				}

				// 重命名
				if err := tx.Exec(`ALTER TABLE models_old RENAME TO models`).Error; err != nil {
					return err
				}

				// 重建索引
				if err := tx.Exec(`CREATE UNIQUE INDEX idx_models_name ON models(name)`).Error; err != nil {
					return err
				}
			} else {
				// MySQL/PostgreSQL: 修改回整数类型
				if err := tx.Exec(`ALTER TABLE models MODIFY COLUMN provider_id INTEGER`).Error; err != nil {
					return err
				}
			}

			return nil
		},
	}
}
