package proxy

import (
	"net/http"
	"strconv"
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
func (fc *FailureClassifier) ClassifyHTTPError(statusCode int) (FailureType, string) {
	// 2xx 成功响应
	if statusCode >= 200 && statusCode < 300 {
		return FailureTypeNone, "正常（" + strconv.Itoa(statusCode) + "）"
	}

	// 硬故障: 认证和授权问题
	if statusCode == http.StatusUnauthorized || // 401
		statusCode == http.StatusPaymentRequired || // 402
		statusCode == http.StatusForbidden { // 403
		return FailureTypeSoft, "认证和授权问题（" + strconv.Itoa(statusCode) + "）"
	}

	// 429 需要检查是否为 quota exceeded
	if statusCode == http.StatusTooManyRequests { // 429
		return FailureTypeSoft, "密钥余额或请求速率限制（" + strconv.Itoa(statusCode) + "）"
	}

	// 404 模型不存在，需要重试但不记录到熔断器
	if statusCode == http.StatusNotFound {
		return FailureTypeModelNotFound, "上游渠道（" + strconv.Itoa(statusCode) + "）"
	}

	// 4xx 客户端错误(除了上述的)
	// 对于渠道代理来说，4xx 应该触发重试（切换到其他渠道）
	// 但不应禁用 Key，因为这不是 Key 的问题
	if statusCode >= 400 && statusCode < 500 {
		return FailureTypeSoft, "代理客户端故障（" + strconv.Itoa(statusCode) + "）"
	}

	// 5xx 服务端错误,软故障
	if statusCode >= 500 && statusCode < 600 {
		return FailureTypeSoft, "渠道接口故障（" + strconv.Itoa(statusCode) + "）"
	}

	// 其他情况
	return FailureTypeNone, "正常（" + strconv.Itoa(statusCode) + "）"
}

// ClassifyNetworkError 分类网络错误
// timeout, connection refused, DNS 等错误归类为软故障
func (fc *FailureClassifier) ClassifyNetworkError(err error) (FailureType, string) {
	if err == nil {
		return FailureTypeNone, "正常"
	}
	return FailureTypeSoft, "网络异常：" + err.Error()
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
		return fc.ClassifyHTTPError(resp.StatusCode)
	}

	return FailureTypeNone, "正常"
}
