package migration

import (
	"gorm.io/gorm"
)

// V1_5_0_AddTotalResponseTime 添加 total_response_time 字段到 request_logs 表
// 用于记录从请求开始到完成的总时间（包括所有重试时间）
func V1_5_0_AddTotalResponseTime(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE request_logs
		ADD COLUMN IF NOT EXISTS total_response_time INT
	`).Error
}
