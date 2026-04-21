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
	Value     string    `gorm:"type:text;not null" json:"value"`
	ValueType string    `gorm:"type:varchar(20);not null;default:'string'" json:"value_type"` // string, int, bool, json
	Remark    string    `gorm:"type:varchar(200)" json:"remark"`
}

// TableName 指定表名
func (SystemSetting) TableName() string {
	return "system_settings"
}

// 预定义的系统设置 Key
const (
	// Server
	SettingServerPort           = "server_port"
	SettingServerReadTimeout    = "server_read_timeout"
	SettingServerWriteTimeout   = "server_write_timeout"
	SettingServerMaxHeaderBytes = "server_max_header_bytes"

	// Logging
	SettingLogAddSource                   = "log_add_source"
	SettingLogFileEnabled                 = "log_file_enabled"
	SettingLogFileMaxSize                 = "log_file_max_size"
	SettingLogFileMaxBackups              = "log_file_max_backups"
	SettingLogFileMaxAge                  = "log_file_max_age"
	SettingLogFileCompress                = "log_file_compress"
	SettingCircuitBreakerFailureThreshold = "circuit_breaker_failure_threshold"
	SettingCircuitBreakerCoolingDuration  = "circuit_breaker_cooling_duration"
	SettingLogRetentionDays               = "log_retention_days"
	SettingLogDebugEnabled                = "log_debug_enabled"
	SettingProxyRequestTimeout            = "proxy_request_timeout"
	SettingProxyNetworkURL                = "proxy_network_url"
	SettingProxyMaxRetry                  = "proxy_max_retry"
	SettingSnifferEnabled                 = "sniffer_enabled"
	SettingSnifferStreamPacketCount       = "sniffer_stream_packet_count"
	SettingSnifferPlainTextErrorRules     = "sniffer_plain_text_error_rules"
	SettingModelTestPrompt                = "model_test_prompt"
	SettingModelTestUserAgent             = "model_test_user_agent"
)

const DefaultModelTestUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) CherryStudio/1.7.13 Chrome/140.0.7339.249 Electron/38.7.0 Safari/537.36"
