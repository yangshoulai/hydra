package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// v1_0_1_optimize_indexes 优化 RequestLog 表索引
func v1_0_1_optimize_indexes() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "104012101-request-log-optimization",
		Migrate: func(tx *gorm.DB) error {
			// 为 RequestLog 表添加复合索引以优化常见查询

			// 索引1: trace_id + created_at (用于按 TraceID 查询和排序)
			if err := tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_request_logs_trace_id_created_at
				ON request_logs(trace_id, created_at DESC)
			`).Error; err != nil {
				return err
			}

			// 索引2: access_token + created_at (用于按 Token 查询请求历史)
			if err := tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_request_logs_access_token_created_at
				ON request_logs(access_token, created_at DESC)
			`).Error; err != nil {
				return err
			}

			// 索引3: channel_id + created_at (用于按渠道查询统计)
			if err := tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_request_logs_channel_id_created_at
				ON request_logs(channel_id, created_at DESC)
			`).Error; err != nil {
				return err
			}

			// 索引4: requested_model + created_at (用于按模型查询统计)
			if err := tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_request_logs_requested_model_created_at
				ON request_logs(requested_model, created_at DESC)
			`).Error; err != nil {
				return err
			}

			// 索引5: status_code + created_at (用于按状态码查询失败请求)
			if err := tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_request_logs_status_code_created_at
				ON request_logs(status_code, created_at DESC)
			`).Error; err != nil {
				return err
			}

			// 索引6: is_success + created_at (用于快速统计成功/失败率)
			if err := tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_request_logs_is_success_created_at
				ON request_logs(is_success, created_at DESC)
			`).Error; err != nil {
				return err
			}

			// 索引7: created_at DESC (用于时间范围查询，支持仪表盘数据聚合)
			if err := tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_request_logs_created_at_desc
				ON request_logs(created_at DESC)
			`).Error; err != nil {
				return err
			}

			// 索引8: channel_id + status_code + created_at (复合索引用于渠道成功率统计)
			if err := tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_request_logs_channel_status_created_at
				ON request_logs(channel_id, status_code, created_at DESC)
			`).Error; err != nil {
				return err
			}

			// 索引9: is_stream + created_at (用于流式请求查询)
			if err := tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_request_logs_is_stream_created_at
				ON request_logs(is_stream, created_at DESC)
			`).Error; err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 删除所有创建的索引
			indexes := []string{
				"idx_request_logs_trace_id_created_at",
				"idx_request_logs_access_token_created_at",
				"idx_request_logs_channel_id_created_at",
				"idx_request_logs_requested_model_created_at",
				"idx_request_logs_status_code_created_at",
				"idx_request_logs_is_success_created_at",
				"idx_request_logs_created_at_desc",
				"idx_request_logs_channel_status_created_at",
				"idx_request_logs_is_stream_created_at",
			}

			for _, index := range indexes {
				if err := tx.Exec(`DROP INDEX IF EXISTS ` + index).Error; err != nil {
					return err
				}
			}

			return nil
		},
	}
}
