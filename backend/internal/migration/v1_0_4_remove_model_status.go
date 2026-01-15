package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// v1_0_4_remove_model_status 移除 models 表的 status 字段
func v1_0_4_remove_model_status() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.0.4-remove-model-status",
		Migrate: func(tx *gorm.DB) error {
			// 检查 models 表是否有 status 列
			type Model struct {
				Status string
			}
			hasStatusColumn := tx.Migrator().HasColumn(&Model{}, "status")

			// 如果 status 列不存在，说明是全新安装，直接返回
			if !hasStatusColumn {
				return nil
			}

			// 删除 status 字段和相关索引
			// SQLite 不支持直接删除列，需要重建表
			if tx.Dialector.Name() == "sqlite" {
				// SQLite: 重建表
				if err := tx.Exec(`
					CREATE TABLE models_new (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						name VARCHAR(100) NOT NULL,
						provider_id INTEGER,
						remark VARCHAR(500),
						created_at DATETIME,
						updated_at DATETIME,
						deleted_at DATETIME
					)
				`).Error; err != nil {
					return err
				}

				// 复制数据
				if err := tx.Exec(`
					INSERT INTO models_new (id, name, provider_id, remark, created_at, updated_at, deleted_at)
					SELECT id, name, provider_id, remark, created_at, updated_at, deleted_at
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

				// 重建索引
				if err := tx.Exec(`CREATE UNIQUE INDEX idx_models_name ON models(name)`).Error; err != nil {
					return err
				}

				if err := tx.Exec(`CREATE INDEX idx_models_deleted_at ON models(deleted_at)`).Error; err != nil {
					return err
				}
			} else {
				// MySQL/PostgreSQL: 直接删除列
				if err := tx.Exec(`ALTER TABLE models DROP COLUMN status`).Error; err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 回滚：添加 status 字段
			if tx.Dialector.Name() == "sqlite" {
				// SQLite: 重建表
				if err := tx.Exec(`
					CREATE TABLE models_new (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						name VARCHAR(100) NOT NULL,
						provider VARCHAR(50) NOT NULL,
						status VARCHAR(20) NOT NULL DEFAULT 'enabled',
						remark VARCHAR(500),
						created_at DATETIME,
						updated_at DATETIME,
						deleted_at DATETIME
					)
				`).Error; err != nil {
					return err
				}

				// 复制数据
				if err := tx.Exec(`
					INSERT INTO models_new (id, name, provider, status, remark, created_at, updated_at, deleted_at)
					SELECT id, name, provider, 'enabled', remark, created_at, updated_at, deleted_at
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

				// 重建索引
				if err := tx.Exec(`CREATE UNIQUE INDEX idx_models_name ON models(name)`).Error; err != nil {
					return err
				}

				if err := tx.Exec(`CREATE INDEX idx_models_status ON models(status)`).Error; err != nil {
					return err
				}

				if err := tx.Exec(`CREATE INDEX idx_models_deleted_at ON models(deleted_at)`).Error; err != nil {
					return err
				}
			} else {
				// MySQL/PostgreSQL: 添加列
				if err := tx.Exec(`ALTER TABLE models ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'enabled'`).Error; err != nil {
					return err
				}

				// 创建索引
				if err := tx.Exec(`CREATE INDEX idx_models_status ON models(status)`).Error; err != nil {
					return err
				}
			}

			return nil
		},
	}
}
