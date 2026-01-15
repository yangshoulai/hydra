package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// v1_0_5_remove_model_deleted_at 移除 models 表的 deleted_at 字段
func v1_0_5_remove_model_deleted_at() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.0.5-remove-model-deleted-at",
		Migrate: func(tx *gorm.DB) error {
			// 检查 models 表是否有 deleted_at 列
			type Model struct {
				DeletedAt interface{}
			}
			hasDeletedAtColumn := tx.Migrator().HasColumn(&Model{}, "deleted_at")

			// 如果 deleted_at 列不存在，说明是全新安装，直接返回
			if !hasDeletedAtColumn {
				return nil
			}

			// 删除 deleted_at 字段和相关索引
			if tx.Dialector.Name() == "sqlite" {
				// SQLite: 先删除索引
				if err := tx.Exec(`DROP INDEX IF EXISTS idx_models_deleted_at`).Error; err != nil {
					return err
				}

				// 重建表
				if err := tx.Exec(`
					CREATE TABLE models_new (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						name VARCHAR(100) NOT NULL,
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
					INSERT INTO models_new (id, name, provider_id, remark, created_at, updated_at)
					SELECT id, name, provider_id, remark, created_at, updated_at
					FROM models
					WHERE deleted_at IS NULL
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
				// MySQL/PostgreSQL: 直接删除列
				if err := tx.Exec(`ALTER TABLE models DROP COLUMN deleted_at`).Error; err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 回滚：添加 deleted_at 字段
			if tx.Dialector.Name() == "sqlite" {
				// SQLite: 重建表
				if err := tx.Exec(`
					CREATE TABLE models_new (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						name VARCHAR(100) NOT NULL,
						provider VARCHAR(50) NOT NULL,
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
					INSERT INTO models_new (id, name, provider, remark, created_at, updated_at, deleted_at)
					SELECT id, name, provider, remark, created_at, updated_at, NULL
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
				// MySQL/PostgreSQL: 添加列
				if err := tx.Exec(`ALTER TABLE models ADD COLUMN deleted_at DATETIME`).Error; err != nil {
					return err
				}

				// 创建索引
				if err := tx.Exec(`CREATE INDEX idx_models_deleted_at ON models(deleted_at)`).Error; err != nil {
					return err
				}
			}

			return nil
		},
	}
}
