package models

import (
	"time"

	"gorm.io/gorm"
)

// SystemSetting 系统设置模型
type SystemSetting struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Key       string         `gorm:"type:varchar(100);not null;uniqueIndex" json:"key"`
	Value     string         `gorm:"type:text;not null" json:"value"`
	ValueType string         `gorm:"type:varchar(20);not null;default:'string'" json:"value_type"` // string, int, bool, json
	Category  string         `gorm:"type:varchar(50);not null;index" json:"category"` // circuit_breaker, logging, proxy, etc.
	Remark    string         `gorm:"type:varchar(200)" json:"remark"`
}

// TableName 指定表名
func (SystemSetting) TableName() string {
	return "system_settings"
}

// 预定义的系统设置 Key
const (
	SettingCircuitBreakerFailureThreshold = "circuit_breaker.failure_threshold"
	SettingCircuitBreakerCoolingDuration  = "circuit_breaker.cooling_duration_sec"
	SettingCircuitBreakerMaxRetry         = "circuit_breaker.max_retry"
	SettingLogRetentionDays               = "log.retention_days"
	SettingLogDebugEnabled                = "log.debug_enabled"
	SettingProxyRequestTimeout            = "proxy.request_timeout"
	SettingProxyMaxResponseSize           = "proxy.max_response_size"
	SettingProxyMaxConcurrent             = "proxy.max_concurrent"
)
