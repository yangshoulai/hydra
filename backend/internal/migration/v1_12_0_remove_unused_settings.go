package migration

import (
	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// V1_12_0_RemoveUnusedSettings 清理已废弃的系统设置
func V1_12_0_RemoveUnusedSettings(db *gorm.DB) error {
	keys := []string{
		"circuit_breaker_probe_interval",
		"circuit_breaker_probe_max_concurrent",
		"proxy_max_response_size",
	}

	return db.Where("key IN ?", keys).Delete(&models.SystemSetting{}).Error
}
