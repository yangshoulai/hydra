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
	// Security
	SettingSecurityJWTSecret = "security_jwt_secret"

	// Server
	SettingServerPort           = "server_port"
	SettingServerReadTimeout    = "server_read_timeout"
	SettingServerWriteTimeout   = "server_write_timeout"
	SettingServerMaxHeaderBytes = "server_max_header_bytes"

	// Logging
	SettingLogAddSource                    = "log_add_source"
	SettingLogFileEnabled                  = "log_file_enabled"
	SettingLogFileMaxSize                  = "log_file_max_size"
	SettingLogFileMaxBackups               = "log_file_max_backups"
	SettingLogFileMaxAge                   = "log_file_max_age"
	SettingLogFileCompress                 = "log_file_compress"
	SettingCircuitBreakerFailureThreshold  = "circuit_breaker_failure_threshold"
	SettingCircuitBreakerCoolingDuration   = "circuit_breaker_cooling_duration"
	SettingLogRetentionDays                = "log_retention_days"
	SettingLogDebugEnabled                 = "log_debug_enabled"
	SettingProxyRequestTimeout             = "proxy_request_timeout"
	SettingProxyKeepaliveInterval          = "proxy_keepalive_interval"
	SettingProxyNonStreamKeepaliveEnabled  = "proxy_non_stream_keepalive_enabled"
	SettingProxyNonStreamKeepaliveDelay    = "proxy_non_stream_keepalive_first_delay"
	SettingProxyNonStreamKeepaliveInterval = "proxy_non_stream_keepalive_interval"
	SettingProxyNetworkURL                 = "proxy_network_url"
	SettingProxyMaxRetry                   = "proxy_max_retry"
	SettingProxyLoadBalanceStrategy        = "proxy_load_balance_strategy"
	SettingProxyMaxBodyBytes               = "proxy_max_body_bytes"
	SettingProxyRateLimitEnabled           = "proxy_rate_limit_enabled"
	SettingProxyRateLimitGlobalRPS         = "proxy_rate_limit_global_rps"
	SettingProxyRateLimitGlobalBurst       = "proxy_rate_limit_global_burst"
	SettingProxyRateLimitTokenRPS          = "proxy_rate_limit_token_rps"
	SettingProxyRateLimitTokenBurst        = "proxy_rate_limit_token_burst"
	SettingSnifferEnabled                  = "sniffer_enabled" // 旧版全局嗅探开关，仅用于迁移兼容
	SettingSnifferNonStreamEnabled         = "sniffer_non_stream_enabled"
	SettingSnifferStreamEnabled            = "sniffer_stream_enabled"
	SettingSnifferStreamPacketCount        = "sniffer_stream_packet_count"
	SettingSnifferPlainTextErrorRules      = "sniffer_plain_text_error_rules"
	SettingModelTestPrompt                 = "model_test_prompt"
	SettingModelTestUserAgent              = "model_test_user_agent"

	// Notification
	SettingNotificationEnabled          = "notification_enabled"
	SettingNotificationChannel          = "notification_channel"
	SettingNotificationEvents           = "notification_events"
	SettingNotificationTelegramBotToken = "notification_telegram_bot_token"
	SettingNotificationTelegramChatID   = "notification_telegram_chat_id"
)

const DefaultModelTestUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) CherryStudio/1.7.13 Chrome/140.0.7339.249 Electron/38.7.0 Safari/537.36"

const (
	ProxyLoadBalanceStrategyWeightedRandom = "weighted_random"
	ProxyLoadBalanceStrategyRoundRobin     = "round_robin"
)

const (
	DefaultProxyMaxBodyBytes                      = 50 * 1024 * 1024
	DefaultProxyRateLimitGlobalRPS                = 300
	DefaultProxyRateLimitGlobalBurst              = 600
	DefaultProxyRateLimitTokenRPS                 = 60
	DefaultProxyRateLimitTokenBurst               = 120
	DefaultProxyNonStreamKeepaliveDelaySeconds    = 80
	DefaultProxyNonStreamKeepaliveIntervalSeconds = 30
)

const (
	NotificationChannelTelegram = "telegram"
)

const (
	NotificationEventCircuitBreaker      = "circuit_breaker"
	NotificationEventAdminLogin          = "admin_login"
	NotificationEventAdminPasswordChange = "admin_password_change"
)
