package models

import (
	"time"

	"gorm.io/gorm"
)

// RequestLog 请求日志模型
type RequestLog struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	TraceID       string         `gorm:"type:varchar(36);not null;index" json:"trace_id"`
	AccessToken   string         `gorm:"type:varchar(64);index" json:"access_token"`
	RequestPath   string         `gorm:"type:varchar(500);not null;index" json:"request_path"`
	RequestMethod string         `gorm:"type:varchar(10);not null" json:"request_method"`

	// 模型信息
	RequestedModel string `gorm:"type:varchar(100);index" json:"requested_model"`
	UnifiedModel   string `gorm:"type:varchar(100);index" json:"unified_model"`
	UpstreamModel  string `gorm:"type:varchar(100)" json:"upstream_model"`

	// 渠道和 Key 信息
	ChannelID    *uint   `gorm:"index" json:"channel_id"`
	ChannelName  string  `gorm:"type:varchar(100)" json:"channel_name"`
	KeyID        *uint   `json:"key_id"`

	// 响应信息
	StatusCode      int       `gorm:"not null;index" json:"status_code"`
	ResponseTime    int       `gorm:"not null" json:"response_time"` // 毫秒
	IsSuccess       bool      `gorm:"not null;index" json:"is_success"`
	ErrorMessage    string    `gorm:"type:text" json:"error_message"`
	RetryCount      int       `gorm:"not null;default:0" json:"retry_count"`

	// 流式响应相关
	IsStream        bool      `gorm:"not null;default:false" json:"is_stream"`
	StreamChunks    int       `gorm:"default:0" json:"stream_chunks"`

	// 客户端信息
	ClientIP        string    `gorm:"type:varchar(50)" json:"client_ip"`
	UserAgent       string    `gorm:"type:varchar(500)" json:"user_agent"`

	// 调试信息（仅在调试模式启用时记录）
	RequestBody     string    `gorm:"type:text" json:"request_body,omitempty"`
	ResponseBody    string    `gorm:"type:text" json:"response_body,omitempty"`
}

// TableName 指定表名
func (RequestLog) TableName() string {
	return "request_logs"
}
