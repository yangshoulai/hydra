package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// v1_1_3_add_token_name_unique_index 添加 access_tokens 表的 name 字段唯一索引
func v1_1_3_add_token_name_unique_index() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.1.3-add-token-name-unique-index",
		Migrate: func(tx *gorm.DB) error {
			// SQLite 不支持直接添加唯一索引到已有数据的表
			// 需要重建表
			if tx.Dialector.Name() == "sqlite" {
				// 0. 清理可能存在的临时表
				tx.Exec(`DROP TABLE IF EXISTS access_tokens_new`)

				// 1. 先处理重复的名称（保留最早的，删除后面的）
				if err := tx.Exec(`
					DELETE FROM access_tokens
					WHERE id NOT IN (
						SELECT MIN(id) FROM access_tokens GROUP BY name
					)
				`).Error; err != nil {
					return err
				}

				// 2. 创建新表结构
				if err := tx.Exec(`
					CREATE TABLE access_tokens_new (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						created_at DATETIME,
						updated_at DATETIME,
						deleted_at DATETIME,
						token_hash VARCHAR(64) NOT NULL UNIQUE,
						token_preview VARCHAR(20),
						status VARCHAR(20) NOT NULL DEFAULT 'active',
						name VARCHAR(20) NOT NULL UNIQUE,
						last_used_at DATETIME,
						expires_at DATETIME
					)
				`).Error; err != nil {
					return err
				}

				// 3. 复制数据
				if err := tx.Exec(`
					INSERT INTO access_tokens_new (id, created_at, updated_at, deleted_at, token_hash, token_preview, status, name, last_used_at, expires_at)
					SELECT id, created_at, updated_at, deleted_at, token_hash, token_preview, status, name, last_used_at, expires_at
					FROM access_tokens
				`).Error; err != nil {
					return err
				}

				// 4. 删除旧表
				if err := tx.Exec(`DROP TABLE access_tokens`).Error; err != nil {
					return err
				}

				// 5. 重命名新表
				if err := tx.Exec(`ALTER TABLE access_tokens_new RENAME TO access_tokens`).Error; err != nil {
					return err
				}

				// 6. 重建索引
				if err := tx.Exec(`CREATE INDEX idx_access_tokens_deleted_at ON access_tokens(deleted_at)`).Error; err != nil {
					return err
				}

			} else {
				// MySQL/PostgreSQL: 直接创建唯一索引
				// 先处理重复的名称（保留最早的，删除后面的）
				if err := tx.Exec(`
					DELETE t1 FROM access_tokens t1
					INNER JOIN access_tokens t2
					WHERE t1.id > t2.id AND t1.name = t2.name
				`).Error; err != nil {
					return err
				}

				// 添加唯一索引
				if err := tx.Exec(`CREATE UNIQUE INDEX idx_access_tokens_name ON access_tokens(name)`).Error; err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// SQLite 回滚
			if tx.Dialector.Name() == "sqlite" {
				// 0. 清理可能存在的临时表
				tx.Exec(`DROP TABLE IF EXISTS access_tokens_old`)

				// 1. 重建表，去掉 name 的唯一约束
				if err := tx.Exec(`
					CREATE TABLE access_tokens_old (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						created_at DATETIME,
						updated_at DATETIME,
						deleted_at DATETIME,
						token_hash VARCHAR(64) NOT NULL UNIQUE,
						token_preview VARCHAR(20),
						status VARCHAR(20) NOT NULL DEFAULT 'active',
						name VARCHAR(20) NOT NULL,
						last_used_at DATETIME,
						expires_at DATETIME
					)
				`).Error; err != nil {
					return err
				}

				// 2. 复制数据
				if err := tx.Exec(`
					INSERT INTO access_tokens_old (id, created_at, updated_at, deleted_at, token_hash, token_preview, status, name, last_used_at, expires_at)
					SELECT id, created_at, updated_at, deleted_at, token_hash, token_preview, status, name, last_used_at, expires_at
					FROM access_tokens
				`).Error; err != nil {
					return err
				}

				// 3. 删除新表
				if err := tx.Exec(`DROP TABLE access_tokens`).Error; err != nil {
					return err
				}

				// 4. 重命名
				if err := tx.Exec(`ALTER TABLE access_tokens_old RENAME TO access_tokens`).Error; err != nil {
					return err
				}

				// 5. 重建索引
				if err := tx.Exec(`CREATE INDEX idx_access_tokens_deleted_at ON access_tokens(deleted_at)`).Error; err != nil {
					return err
				}

			} else {
				// MySQL/PostgreSQL: 删除唯一索引
				if err := tx.Exec(`DROP INDEX idx_access_tokens_name ON access_tokens`).Error; err != nil {
					return err
				}
			}

			return nil
		},
	}
}
