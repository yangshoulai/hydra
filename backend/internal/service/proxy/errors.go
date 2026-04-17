package proxy

import "errors"

var (
	// ErrModelNotFound 统一模型不存在
	ErrModelNotFound = errors.New("model not found")
	// ErrNoAvailableChannel 无可用渠道
	ErrNoAvailableChannel = errors.New("no available channel")
	// ErrNoAvailableRoute 无可用路由
	ErrNoAvailableRoute = errors.New("no available route for request")
	// ErrAllUpstreamAttemptsFailed 所有上游重试都失败
	ErrAllUpstreamAttemptsFailed = errors.New("all upstream attempts failed")
	// ErrReadRequestBodyFailed 读取请求体失败
	ErrReadRequestBodyFailed = errors.New("failed to read request body")
	// ErrUpstreamCallFailed 上游调用失败
	ErrUpstreamCallFailed = errors.New("upstream call failed")
	// ErrFake200Response 假 200 响应
	ErrFake200Response = errors.New("fake 200 response")
	// ErrEmptySniffResult 嗅探响应为空
	ErrEmptySniffResult = errors.New("empty sniff result")
	// ErrResponseValidationFailed 响应校验失败
	ErrResponseValidationFailed = errors.New("response validation failed")
	// ErrModelNotAllowedForToken 令牌无权访问模型
	ErrModelNotAllowedForToken = errors.New("model not allowed for token")
)

// RetryableProxyError 可重试的代理错误
type RetryableProxyError struct {
	Cause        error
	FailureType  FailureType
	FailureScope FailureScope
	Stage        string
}

func (e *RetryableProxyError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Stage
}

func (e *RetryableProxyError) Unwrap() error {
	return e.Cause
}

// NewRetryableProxyError 创建可重试错误
func NewRetryableProxyError(cause error, failureType FailureType, failureScope FailureScope, stage string) *RetryableProxyError {
	return &RetryableProxyError{
		Cause:        cause,
		FailureType:  failureType,
		FailureScope: failureScope,
		Stage:        stage,
	}
}

// AsRetryableProxyError 判断是否为可重试错误
func AsRetryableProxyError(err error) (*RetryableProxyError, bool) {
	var retryErr *RetryableProxyError
	if !errors.As(err, &retryErr) {
		return nil, false
	}
	return retryErr, true
}

type detailedProxyError struct {
	base   error
	detail string
}

func (e *detailedProxyError) Error() string {
	if e.detail == "" {
		return e.base.Error()
	}
	return e.base.Error() + ": " + e.detail
}

func (e *detailedProxyError) Unwrap() error {
	return e.base
}

// NewUpstreamCallError 创建上游调用错误
func NewUpstreamCallError(detail string) error {
	if detail == "" {
		return ErrUpstreamCallFailed
	}
	return &detailedProxyError{
		base:   ErrUpstreamCallFailed,
		detail: detail,
	}
}

// NewResponseValidationError 创建响应校验错误
func NewResponseValidationError(detail string) error {
	if detail == "" {
		return ErrResponseValidationFailed
	}
	return &detailedProxyError{
		base:   ErrResponseValidationFailed,
		detail: detail,
	}
}

// EmptySSEBodyError 空响应体错误（用于触发重试）
type EmptySSEBodyError struct {
	TraceID string
	Message string
}

func (e *EmptySSEBodyError) Error() string {
	return e.Message
}

func (e *EmptySSEBodyError) Timeout() bool {
	return false
}

func (e *EmptySSEBodyError) Temporary() bool {
	return false
}
