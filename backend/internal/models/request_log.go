package models

import (
	"time"

	"gorm.io/gorm"
)

// RequestLogMain 请求日志主表：记录客户端请求的整体信息
type RequestLogMain struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 追踪标识
	TraceID string `gorm:"type:varchar(36);not null;uniqueIndex;index" json:"trace_id"`

	// 请求基本信息
	EndpointType   string `gorm:"type:varchar(50);index" json:"endpoint_type"` // chat, completions, embeddings, etc.
	RequestPath    string `gorm:"type:varchar(500);not null;index" json:"request_path"`
	RequestMethod  string `gorm:"type:varchar(10);not null" json:"request_method"`
	RequestedModel string `gorm:"type:varchar(100);index" json:"requested_model"`
	UnifiedModel   string `gorm:"type:varchar(100);index" json:"unified_model"`

	// 客户端信息
	AccessToken string `gorm:"type:varchar(64);index" json:"access_token"`
	ClientIP    string `gorm:"type:varchar(50)" json:"client_ip"`
	UserAgent   string `gorm:"type:varchar(500)" json:"user_agent"`

	// 时间信息
	StartTime      time.Time `gorm:"not null;index" json:"start_time"`
	EndTime        time.Time `gorm:"not null" json:"end_time"`
	Duration       int       `gorm:"not null" json:"duration"` // 总耗时（毫秒）

	// 最终结果
	IsSuccess   bool  `gorm:"not null;index" json:"is_success"`
	StatusCode  int   `gorm:"not null;index" json:"status_code"`
	RetryCount  int   `gorm:"not null;default:0" json:"retry_count"`
	IsStream    bool  `gorm:"not null;default:false;index" json:"is_stream"`

	// 最后成功/失败的渠道信息
	LastChannelID   *uint  `gorm:"index" json:"last_channel_id,omitempty"`
	LastChannelName string `gorm:"type:varchar(100)" json:"last_channel_name,omitempty"`
	LastModel       string `gorm:"type:varchar(100)" json:"last_model,omitempty"`

	// 错误信息（记录在代理开始前或整体失败的错误）
	ErrorMessage string `gorm:"type:text" json:"error_message,omitempty"`

	// 关联明细记录
	Details []RequestLogDetail `gorm:"foreignKey:MainLogID" json:"details"`
}

// AfterFind GORM 钩子：查询后自动填充本地时间字段
func (rlm *RequestLogMain) AfterFind(tx *gorm.DB) error {
	return nil
}

// TableName 指定表名
func (RequestLogMain) TableName() string {
	return "request_logs_main"
}

// RequestLogDetail 请求日志明细表：记录每次重试的详细信息
type RequestLogDetail struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	// 关联主表
	MainLogID uint `gorm:"not null;index:idx_main_log_retry" json:"main_log_id"`

	// 渠道和模型信息
	ChannelID   *uint  `gorm:"index" json:"channel_id,omitempty"`
	ChannelName string `gorm:"type:varchar(100);index" json:"channel_name"`
	Model       string `gorm:"type:varchar(100)" json:"model"`

	// 密钥信息
	KeyID *uint `json:"key_id,omitempty"`

	// 时间信息
	StartTime time.Time `gorm:"not null" json:"start_time"`
	EndTime   time.Time `gorm:"not null" json:"end_time"`
	Duration  int       `gorm:"not null" json:"duration"` // 本次尝试耗时（毫秒）

	// 请求和响应信息
	RequestBodySize  int    `gorm:"default:0" json:"request_body_size"`
	ResponseBodySize int    `gorm:"default:0" json:"response_body_size"`

	// 状态信息
	StatusCode int    `gorm:"not null" json:"status_code"`
	IsSuccess  bool   `gorm:"not null" json:"is_success"`
	Status     string `gorm:"type:varchar(50)" json:"status"` // success, failed, timeout, etc.
	RetryIndex int    `gorm:"not null;index:idx_main_log_retry" json:"retry_index"` // 第几次重试（0表示首次尝试）

	// 流式响应信息
	IsStream      bool `gorm:"not null;default:false" json:"is_stream"`
	StreamChunks  int  `gorm:"default:0" json:"stream_chunks"`
	StreamFirstChunkTime *int `json:"stream_first_chunk_time,omitempty"` // 首帧响应时间（毫秒）

	// 详细信息（仅在调试模式启用时记录）
	RequestHeaders  string `gorm:"type:text" json:"request_headers,omitempty"`
	RequestBody     string `gorm:"type:text" json:"request_body,omitempty"`
	ResponseHeaders string `gorm:"type:text" json:"response_headers,omitempty"`
	ResponseBody    string `gorm:"type:text" json:"response_body,omitempty"`

	// 错误信息
	ErrorMessage string `gorm:"type:text" json:"error_message,omitempty"`
}

// AfterFind GORM 钩子
func (rld *RequestLogDetail) AfterFind(tx *gorm.DB) error {
	return nil
}

// TableName 指定表名
func (RequestLogDetail) TableName() string {
	return "request_logs_detail"
}
