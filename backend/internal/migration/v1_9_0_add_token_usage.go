package migration

import "gorm.io/gorm"

// V1_9_0_AddTokenUsage 添加 token 统计字段
func V1_9_0_AddTokenUsage(db *gorm.DB) error {
	statements := []string{
		`ALTER TABLE request_logs_main ADD COLUMN IF NOT EXISTS prompt_tokens BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE request_logs_main ADD COLUMN IF NOT EXISTS completion_tokens BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE request_logs_detail ADD COLUMN IF NOT EXISTS prompt_tokens BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE request_logs_detail ADD COLUMN IF NOT EXISTS completion_tokens BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE channel_model_configs ADD COLUMN IF NOT EXISTS prompt_tokens BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE channel_model_configs ADD COLUMN IF NOT EXISTS completion_tokens BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE keys ADD COLUMN IF NOT EXISTS prompt_tokens BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE keys ADD COLUMN IF NOT EXISTS completion_tokens BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS prompt_tokens BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS completion_tokens BIGINT NOT NULL DEFAULT 0`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	return nil
}
