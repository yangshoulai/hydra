package models

import (
	"time"
)

// SystemSetting 系统设置模型
type SystemSetting struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Key       string    `gorm:"type:varchar(100);not null;uniqueIndex" json:"key"`
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
	SettingCircuitBreakerFailureThreshold = "circuit_breaker_failure_threshold"
	SettingCircuitBreakerCoolingDuration  = "circuit_breaker_cooling_duration"
	SettingCircuitBreakerProbeInterval   = "circuit_breaker_probe_interval"
	SettingCircuitBreakerProbeMaxConcurrent = "circuit_breaker_probe_max_concurrent"
	SettingLogRetentionDays               = "log_retention_days"
	SettingLogDebugEnabled                = "log_debug_enabled"
	SettingProxyRequestTimeout            = "proxy_request_timeout"
	SettingProxyMaxResponseSize           = "proxy_max_response_size"
	SettingProxyMaxConcurrent             = "proxy_max_concurrent"
	SettingProxyMaxRetry                  = "proxy_max_retry"
	SettingSnifferPlainTextErrorRules     = "sniffer_plain_text_error_rules"
	SettingSnifferStreamErrorRules        = "sniffer_stream_error_rules"
)
