package migration

import (
	"gorm.io/gorm"
)

// V1_2_0_AddEndpointTypes 添加端点类型字段
func V1_2_0_AddEndpointTypes(db *gorm.DB) error {
	// 给 channel_model_configs 表添加 endpoint_types 字段
	// 使用 TEXT 类型存储 JSON 数组，支持多选
	// 默认值为 '["openai"]'
	if err := db.Exec(`
		ALTER TABLE channel_model_configs
		ADD COLUMN IF NOT EXISTS endpoint_types TEXT DEFAULT '["openai"]'
	`).Error; err != nil {
		return err
	}

	// 更新现有记录的 endpoint_types 为默认值
	return db.Exec(`
		UPDATE channel_model_configs
		SET endpoint_types = '["openai"]'
		WHERE endpoint_types IS NULL OR endpoint_types = ''
	`).Error
}
