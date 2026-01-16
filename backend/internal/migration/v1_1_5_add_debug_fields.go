package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// V1_1_5_AddDebugFields 添加调试模式所需的字段
func V1_1_5_AddDebugFields() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "1.1.5_add_debug_fields",
		Migrate: func(tx *gorm.DB) error {
			// 检查 request_body 字段是否已存在
			if !tx.Migrator().HasColumn(&models.RequestLog{}, "request_body") {
				// 添加 request_body 字段
				if err := tx.Exec(`
					ALTER TABLE request_logs
					ADD COLUMN request_body LONGTEXT NULL;
				`).Error; err != nil {
					return err
				}
			}

			// 检查 response_body 字段是否已存在
			if !tx.Migrator().HasColumn(&models.RequestLog{}, "response_body") {
				// 添加 response_body 字段
				if err := tx.Exec(`
					ALTER TABLE request_logs
					ADD COLUMN response_body LONGTEXT NULL;
				`).Error; err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 删除添加的字段
			if tx.Migrator().HasColumn(&models.RequestLog{}, "request_body") {
				if err := tx.Migrator().DropColumn(&models.RequestLog{}, "request_body"); err != nil {
					return err
				}
			}

			if tx.Migrator().HasColumn(&models.RequestLog{}, "response_body") {
				if err := tx.Migrator().DropColumn(&models.RequestLog{}, "response_body"); err != nil {
					return err
				}
			}

			return nil
		},
	}
}
