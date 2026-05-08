package models

import "time"

// RequestLog 请求日志主表
//
// 每次代理请求写一行，无论是否开启调试模式。记录请求的核心元信息与最终结果。
// 详细的请求/响应 body 放在 RequestLogDetail，重试轨迹放在 RequestLogAttempt，
// 调试模式关闭时仍写 RequestLogAttempt 基础信息，但不写报文/头。
type RequestLog struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time `gorm:"index;index:idx_logs_channel_time,priority:2;index:idx_logs_model_time,priority:2;index:idx_logs_token_time,priority:2" json:"created_at"`
	TraceID         string    `gorm:"type:varchar(64);not null;uniqueIndex" json:"trace_id"`
	ClientIP        string    `gorm:"type:varchar(64)" json:"client_ip"`
	AccessTokenID   uint      `gorm:"index;index:idx_logs_token_time,priority:1" json:"access_token_id"`
	AccessTokenName string    `gorm:"type:varchar(100)" json:"access_token_name"`
	Method          string    `gorm:"type:varchar(10)" json:"method"`
	Path            string    `gorm:"type:varchar(500)" json:"path"`
	EndpointType    string    `gorm:"type:varchar(50);index" json:"endpoint_type"`
	Model           string    `gorm:"type:varchar(100);index;index:idx_logs_model_time,priority:1" json:"model"`
	IsStream        bool      `json:"is_stream"`
	StatusCode      int       `gorm:"index" json:"status_code"`
	Success         bool      `gorm:"index" json:"success"`
	DurationMS      int64     `json:"duration_ms"`
	RouteAttempts   int       `json:"route_attempts"`
	RetryCount      int       `json:"retry_count"`

	FinalChannelID     uint   `gorm:"index;index:idx_logs_channel_time,priority:1" json:"final_channel_id"`
	FinalChannelName   string `gorm:"type:varchar(100)" json:"final_channel_name"`
	FinalKeyID         uint   `json:"final_key_id"`
	FinalModelConfigID uint   `json:"final_model_config_id"`
	FinalChannelModel  string `gorm:"type:varchar(100)" json:"final_channel_model"`

	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`

	FailureType  string `gorm:"type:varchar(30)" json:"failure_type"`
	FailureScope string `gorm:"type:varchar(30)" json:"failure_scope"`
	FailureStage string `gorm:"type:varchar(50)" json:"failure_stage"`
	ErrorMessage string `gorm:"type:varchar(500)" json:"error_message"`
}

// TableName 指定表名
func (RequestLog) TableName() string {
	return "request_logs"
}

// RequestLogDetail 请求日志 1:1 详情表
//
// 仅在调试模式开启时写入。保存客户端请求头/体、最终响应头/体。
// 敏感头（Authorization / X-Api-Key 等）在入库前已脱敏为 "***"。
type RequestLogDetail struct {
	TraceID   string    `gorm:"type:varchar(64);primaryKey" json:"trace_id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`

	RequestHeadersJSON string `gorm:"type:text" json:"request_headers_json"`
	RequestBody        string `gorm:"type:text" json:"request_body"`
	RequestBodySize    int64  `json:"request_body_size"`

	ResponseHeadersJSON string `gorm:"type:text" json:"response_headers_json"`
	ResponseBody        string `gorm:"type:text" json:"response_body"`
	ResponseBodySize    int64  `json:"response_body_size"`
}

// TableName 指定表名
func (RequestLogDetail) TableName() string {
	return "request_log_details"
}

// RequestLogAttempt 请求日志 1:N 渠道调用表
//
// 每次上游尝试（路由 → 调用 → 响应）一行。
// 调试模式关闭时仅保存渠道、模型、Key、状态、耗时、错误等基础信息；
// 调试模式开启时额外保存上游请求/响应 headers 与 body。
type RequestLogAttempt struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
	TraceID    string    `gorm:"type:varchar(64);not null;uniqueIndex:idx_trace_attempt,priority:1" json:"trace_id"`
	AttemptNum int       `gorm:"uniqueIndex:idx_trace_attempt,priority:2" json:"attempt_num"`

	ChannelID     uint   `json:"channel_id"`
	ChannelName   string `gorm:"type:varchar(100)" json:"channel_name"`
	ModelConfigID uint   `json:"model_config_id"`
	Model         string `gorm:"type:varchar(100)" json:"model"`
	ChannelModel  string `gorm:"type:varchar(100)" json:"channel_model"`
	KeyID         uint   `json:"key_id"`
	KeyName       string `gorm:"type:varchar(200)" json:"key_name"`
	KeyMasked     string `gorm:"type:varchar(64)" json:"key_masked"`

	UpstreamURL        string `gorm:"type:varchar(1000)" json:"upstream_url"`
	DurationMS         int64  `json:"duration_ms"`
	UpstreamStatusCode int    `json:"upstream_status_code"`

	Success      bool   `json:"success"`
	FailureType  string `gorm:"type:varchar(30)" json:"failure_type"`
	FailureScope string `gorm:"type:varchar(30)" json:"failure_scope"`
	FailureStage string `gorm:"type:varchar(50)" json:"failure_stage"`
	ErrorMessage string `gorm:"type:varchar(500)" json:"error_message"`

	UpstreamRequestHeadersJSON string `gorm:"type:text" json:"upstream_request_headers_json"`
	UpstreamRequestBody        string `gorm:"type:text" json:"upstream_request_body"`
	UpstreamRequestBodySize    int64  `json:"upstream_request_body_size"`

	UpstreamResponseHeadersJSON string `gorm:"type:text" json:"upstream_response_headers_json"`
	UpstreamResponseBody        string `gorm:"type:text" json:"upstream_response_body"`
	UpstreamResponseBodySize    int64  `json:"upstream_response_body_size"`
}

// TableName 指定表名
func (RequestLogAttempt) TableName() string {
	return "request_log_attempts"
}
