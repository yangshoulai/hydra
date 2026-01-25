package migration

import "gorm.io/gorm"

// V1_10_0_AddKeyGroups 添加密钥分组字段
func V1_10_0_AddKeyGroups(db *gorm.DB) error {
	statements := []string{
		`ALTER TABLE keys ADD COLUMN IF NOT EXISTS key_group VARCHAR(100) NOT NULL DEFAULT 'Default'`,
		`ALTER TABLE channel_model_configs ADD COLUMN IF NOT EXISTS key_groups TEXT NOT NULL DEFAULT '["Default"]'`,
		`UPDATE keys SET key_group = 'Default' WHERE key_group IS NULL OR key_group = ''`,
		`UPDATE channel_model_configs SET key_groups = '["Default"]' WHERE key_groups IS NULL OR key_groups = ''`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	return nil
}
