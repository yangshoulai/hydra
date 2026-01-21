package migration

import (
	"gorm.io/gorm"
)

// V1_7_0_DropRequestLog 删除旧的 RequestLog 表
func V1_7_0_DropRequestLog(db *gorm.DB) error {
	// 删除旧的 request_logs 表
	if err := db.Migrator().DropTable(&RequestLogOld{}); err != nil {
		return err
	}

	return nil
}

// RequestLogOld 旧的请求日志模型（仅用于迁移）
type RequestLogOld struct {
	ID uint `gorm:"primarykey"`
}

// TableName 指定表名
func (RequestLogOld) TableName() string {
	return "request_logs"
}
