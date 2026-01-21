package migration

import (
	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// V1_6_0_AddRequestLogMainDetail 创建新的请求日志主表和明细表
func V1_6_0_AddRequestLogMainDetail(db *gorm.DB) error {
	// 创建主表和明细表
	if err := db.AutoMigrate(
		&models.RequestLogMain{},
		&models.RequestLogDetail{},
	); err != nil {
		return err
	}

	// 创建额外的索引
	indexes := []string{
		// 主表索引
		`CREATE INDEX IF NOT EXISTS idx_request_logs_main_start_time ON request_logs_main(start_time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_main_access_token_start_time ON request_logs_main(access_token, start_time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_main_requested_model_start_time ON request_logs_main(requested_model, start_time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_main_is_success_start_time ON request_logs_main(is_success, start_time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_main_endpoint_type_start_time ON request_logs_main(endpoint_type, start_time DESC)`,

		// 明细表索引
		`CREATE INDEX IF NOT EXISTS idx_request_logs_detail_channel_id_created_at ON request_logs_detail(channel_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_detail_channel_name_created_at ON request_logs_detail(channel_name, created_at DESC)`,
	}

	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			db.Logger.Warn(nil, "failed to create index", "error", err.Error())
		}
	}

	return nil
}
