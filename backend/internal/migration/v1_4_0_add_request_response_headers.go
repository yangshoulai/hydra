package migration

import (
	"gorm.io/gorm"
)

// V1_4_0_AddRequestResponseHeaders 添加请求头和响应头字段到 request_logs 表
func V1_4_0_AddRequestResponseHeaders(db *gorm.DB) error {
	// 给 request_logs 表添加 request_headers 字段
	if err := db.Exec(`
		ALTER TABLE request_logs
		ADD COLUMN IF NOT EXISTS request_headers TEXT
	`).Error; err != nil {
		return err
	}

	// 给 request_logs 表添加 response_headers 字段
	return db.Exec(`
		ALTER TABLE request_logs
		ADD COLUMN IF NOT EXISTS response_headers TEXT
	`).Error
}
