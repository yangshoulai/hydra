package migration

import "gorm.io/gorm"

// V1_11_0_AddModelConfigCooling 给 channel_model_configs 表添加 cooling_at 字段
func V1_11_0_AddModelConfigCooling(tx *gorm.DB) error {
	statements := []string{
		`ALTER TABLE channel_model_configs ADD COLUMN IF NOT EXISTS cooling_at TIMESTAMP NULL`,
		`CREATE INDEX IF NOT EXISTS idx_channel_model_configs_cooling_at ON channel_model_configs(cooling_at)`,
	}

	for _, stmt := range statements {
		if err := tx.Exec(stmt).Error; err != nil {
			return err
		}
	}

	return nil
}
