package proxy

import (
	"net/http"
	"strings"
)

// FailureType 故障类型
type FailureType string

const (
	// FailureTypeHard 硬故障(永久禁用)
	FailureTypeHard FailureType = "hard"
	// FailureTypeSoft 软故障(熔断后可恢复)
	FailureTypeSoft FailureType = "soft"
	// FailureTypeModelNotFound 模型不存在(需要重试但不记录到熔断器)
	FailureTypeModelNotFound FailureType = "model_not_found"
	// FailureTypeNone 非故障
	FailureTypeNone FailureType = "none"
)

// FailureClassifier 故障分类器
type FailureClassifier struct{}

// NewFailureClassifier 创建故障分类器
func NewFailureClassifier() *FailureClassifier {
	return &FailureClassifier{}
}

// ClassifyHTTPError 分类 HTTP 错误
// 根据状态码和响应内容判断是硬故障还是软故障
func (fc *FailureClassifier) ClassifyHTTPError(statusCode int, body []byte) (FailureType, string) {
	// 2xx 成功响应
	if statusCode >= 200 && statusCode < 300 {
		return FailureTypeNone, ""
	}

	// 硬故障: 认证和授权问题
	if statusCode == http.StatusUnauthorized || // 401
		statusCode == http.StatusPaymentRequired || // 402
		statusCode == http.StatusForbidden { // 403
		return FailureTypeHard, "认证和授权问题"
	}

	// 429 需要检查是否为 quota exceeded
	if statusCode == http.StatusTooManyRequests { // 429
		if fc.isQuotaExceeded(body) {
			return FailureTypeHard, "密钥额度问题"
		}
		// 普通的 rate limit,可能恢复
		return FailureTypeSoft, "请求速率限制"
	}

	// 404 模型不存在，需要重试但不记录到熔断器
	if statusCode == http.StatusNotFound {
		return FailureTypeModelNotFound, "上游渠道 404"
	}

	// 4xx 客户端错误(除了上述的)
	// 对于渠道代理来说，4xx 应该触发重试（切换到其他渠道）
	// 但不应禁用 Key，因为这不是 Key 的问题
	if statusCode >= 400 && statusCode < 500 {
		return FailureTypeSoft, "代理客户端故障"
	}

	// 5xx 服务端错误,软故障
	if statusCode >= 500 && statusCode < 600 {
		return FailureTypeSoft, "渠道接口故障"
	}

	// 其他情况
	return FailureTypeNone, ""
}

// ClassifyNetworkError 分类网络错误
// timeout, connection refused, DNS 等错误归类为软故障
func (fc *FailureClassifier) ClassifyNetworkError(err error) (FailureType, string) {
	if err == nil {
		return FailureTypeNone, ""
	}

	errMsg := strings.ToLower(err.Error())

	// 超时错误
	if strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "deadline exceeded") {
		return FailureTypeSoft, errMsg
	}

	// 连接错误
	if strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "connection reset") ||
		strings.Contains(errMsg, "broken pipe") ||
		strings.Contains(errMsg, "no such host") ||
		strings.Contains(errMsg, "network is unreachable") {
		return FailureTypeSoft, errMsg
	}

	// TLS 错误
	if strings.Contains(errMsg, "tls") ||
		strings.Contains(errMsg, "certificate") {
		return FailureTypeSoft, errMsg
	}

	// EOF 错误
	if strings.Contains(errMsg, "eof") {
		return FailureTypeSoft, errMsg
	}

	// 默认为软故障
	return FailureTypeSoft, errMsg
}

// isQuotaExceeded 检查是否为额度耗尽错误
func (fc *FailureClassifier) isQuotaExceeded(body []byte) bool {
	bodyLower := strings.ToLower(string(body))

	quotaKeywords := []string{
		"quota exceeded",
		"quota_exceeded",
		"insufficient funds",
		"insufficient_funds",
		"insufficient quota",
		"insufficient_quota",
		"billing issue",
		"billing_issue",
		"account suspended",
		"account_suspended",
	}

	for _, keyword := range quotaKeywords {
		if strings.Contains(bodyLower, keyword) {
			return true
		}
	}

	return false
}

// ShouldRetry 判断是否应该重试
// 软故障应该重试,硬故障不应该重试
func (fc *FailureClassifier) ShouldRetry(failureType FailureType) bool {
	return failureType == FailureTypeSoft || failureType == FailureTypeModelNotFound
}

// ClassifyResponseError 综合分类响应错误
// 同时考虑 HTTP 状态码和网络错误
func (fc *FailureClassifier) ClassifyResponseError(resp *http.Response, networkErr error) (FailureType, string) {
	// 优先处理网络错误
	if networkErr != nil {
		return fc.ClassifyNetworkError(networkErr)
	}

	// 处理 HTTP 错误
	if resp != nil {
		return fc.ClassifyHTTPError(resp.StatusCode, nil)
	}

	return FailureTypeNone, ""
}

// GetFailureMessage 获取故障类型的描述信息
func (fc *FailureClassifier) GetFailureMessage(failureType FailureType) string {
	switch failureType {
	case FailureTypeHard:
		return "Hard failure detected - Key will be permanently disabled"
	case FailureTypeSoft:
		return "Soft failure detected - Key will be temporarily cooled down"
	case FailureTypeNone:
		return "No failure detected"
	default:
		return "Unknown failure type"
	}
}

// IsHardFailure 判断是否为硬故障
func (fc *FailureClassifier) IsHardFailure(failureType FailureType) bool {
	return failureType == FailureTypeHard
}

// IsSoftFailure 判断是否为软故障
func (fc *FailureClassifier) IsSoftFailure(failureType FailureType) bool {
	return failureType == FailureTypeSoft
}
