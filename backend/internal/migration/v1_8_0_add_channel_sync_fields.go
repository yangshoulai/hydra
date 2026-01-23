package migration

import "gorm.io/gorm"

// V1_8_0_AddChannelSyncFields 添加渠道同步相关字段
func V1_8_0_AddChannelSyncFields(db *gorm.DB) error {
	if err := db.Exec(`
		ALTER TABLE channels
		ADD COLUMN IF NOT EXISTS last_sync_time TIMESTAMP NULL
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		ALTER TABLE channels
		ADD COLUMN IF NOT EXISTS sync_enabled BOOLEAN DEFAULT true
	`).Error; err != nil {
		return err
	}

	return db.Exec(`
		UPDATE channels
		SET sync_enabled = true
		WHERE sync_enabled IS NULL
	`).Error
}
