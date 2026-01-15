package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
	"github.com/yangshoulai/hydra/internal/models"
)

// V1_0_6_AddAccessTokenExpiresAt 添加访问令牌过期时间字段
func V1_0_6_AddAccessTokenExpiresAt() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "1.0.6_add_access_token_expires_at",
		Migrate: func(tx *gorm.DB) error {
			// 检查 expires_at 字段是否已存在
			if tx.Migrator().HasColumn(&models.AccessToken{}, "expires_at") {
				// 字段已存在，跳过
				return nil
			}

			// 添加 expires_at 字段
			// SQLite 不支持 AFTER 语法，只支持在末尾添加列
			return tx.Exec(`
				ALTER TABLE access_tokens
				ADD COLUMN expires_at DATETIME NULL;
			`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			// SQLite 不支持 DROP COLUMN，需要重建表
			// 这里使用 GORM 的 AutoMigrate 来处理
			return tx.Migrator().DropColumn(&models.AccessToken{}, "expires_at")
		},
	}
}
