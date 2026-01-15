package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// v1_0_2_access_token_updates 更新 access_tokens 表
func v1_0_2_access_token_updates() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.0.2_access_token_updates",
		Migrate: func(tx *gorm.DB) error {
			// 检查是否需要迁移（如果 name 字段已存在，说明已经迁移过了）
			if tx.Migrator().HasColumn(&models.AccessToken{}, "name") {
				// 已经迁移过，只需要确保 token_preview 字段存在
				if !tx.Migrator().HasColumn(&models.AccessToken{}, "token_preview") {
					if err := tx.Exec("ALTER TABLE access_tokens ADD COLUMN token_preview VARCHAR(20)").Error; err != nil {
						return err
					}
				}
				return nil
			}

			// 需要进行迁移：创建新表结构
			if err := tx.Exec(`
				CREATE TABLE access_tokens_new (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					created_at DATETIME,
					updated_at DATETIME,
					deleted_at DATETIME,
					token_hash VARCHAR(64) NOT NULL UNIQUE,
					token_preview VARCHAR(20),
					status VARCHAR(20) NOT NULL DEFAULT 'active',
					name VARCHAR(20) NOT NULL,
					last_used_at DATETIME
				)
			`).Error; err != nil {
				return err
			}

			// 检查 remark 字段是否存在
			hasRemarkColumn := false
			rows, err := tx.Raw("PRAGMA table_info(access_tokens)").Rows()
			if err == nil {
				for rows.Next() {
					var cid int
					var name string
					var ctype string
					var notnull int
					var dfltValue interface{}
					var pk int
					rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk)
					if name == "remark" {
						hasRemarkColumn = true
						break
					}
				}
				rows.Close()
			}

			// 复制数据
			var copySQL string
			if hasRemarkColumn {
				// 旧数据有 remark 字段
				copySQL = `
					INSERT INTO access_tokens_new (id, created_at, updated_at, deleted_at, token_hash, status, name, last_used_at)
					SELECT id, created_at, updated_at, deleted_at, token_hash, status,
					       SUBSTR(remark, 1, 20) as name,
					       last_used_at
					FROM access_tokens
				`
			} else {
				// 旧数据没有 remark 字段，使用空字符串或默认值
				copySQL = `
					INSERT INTO access_tokens_new (id, created_at, updated_at, deleted_at, token_hash, token_preview, status, name, last_used_at)
					SELECT id, created_at, updated_at, deleted_at, token_hash, '', status, '', last_used_at
					FROM access_tokens
				`
			}

			if err := tx.Exec(copySQL).Error; err != nil {
				return err
			}

			// 删除旧表
			if err := tx.Exec("DROP TABLE access_tokens").Error; err != nil {
				return err
			}

			// 重命名新表
			if err := tx.Exec("ALTER TABLE access_tokens_new RENAME TO access_tokens").Error; err != nil {
				return err
			}

			// 重建索引
			if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uix_access_tokens_token_hash ON access_tokens(token_hash)").Error; err != nil {
				return err
			}
			if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_access_tokens_deleted_at ON access_tokens(deleted_at)").Error; err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 回滚操作
			if tx.Dialector.Name() == "sqlite" {
				// SQLite 回滚
				if err := tx.Exec(`
					CREATE TABLE access_tokens_old (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						created_at DATETIME,
						updated_at DATETIME,
						deleted_at DATETIME,
						token_hash VARCHAR(64) NOT NULL UNIQUE,
						status VARCHAR(20) NOT NULL DEFAULT 'active',
						remark VARCHAR(200),
						last_used_at DATETIME
					)
				`).Error; err != nil {
					return err
				}

				if err := tx.Exec(`
					INSERT INTO access_tokens_old (id, created_at, updated_at, deleted_at, token_hash, status, remark, last_used_at)
					SELECT id, created_at, updated_at, deleted_at, token_hash, status, name, last_used_at
					FROM access_tokens
				`).Error; err != nil {
					return err
				}

				if err := tx.Exec("DROP TABLE access_tokens").Error; err != nil {
					return err
				}

				if err := tx.Exec("ALTER TABLE access_tokens_old RENAME TO access_tokens").Error; err != nil {
					return err
				}

				// 重建索引
				if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uix_access_tokens_token_hash ON access_tokens(token_hash)").Error; err != nil {
					return err
				}
				if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_access_tokens_deleted_at ON access_tokens(deleted_at)").Error; err != nil {
					return err
				}

			} else {
				// MySQL/PostgreSQL 回滚
				// 1. 添加 remark 字段
				if err := tx.Exec("ALTER TABLE access_tokens ADD COLUMN remark VARCHAR(200)").Error; err != nil {
					return err
				}

				// 2. 复制数据
				if err := tx.Exec("UPDATE access_tokens SET remark = name").Error; err != nil {
					return err
				}

				// 3. 删除新字段
				if err := tx.Exec("ALTER TABLE access_tokens DROP COLUMN name").Error; err != nil {
					return err
				}
				if err := tx.Exec("ALTER TABLE access_tokens DROP COLUMN token_preview").Error; err != nil {
					return err
				}
			}

			return nil
		},
	}
}
