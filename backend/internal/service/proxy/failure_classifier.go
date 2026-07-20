package proxy

import (
	"net/http"
	"strconv"
	"strings"
)

// FailureType 故障类型
type FailureType string

const (
	// FailureTypeHard 硬故障(永久禁用)
	FailureTypeHard FailureType = "hard"
	// FailureTypeSoft 软故障(熔断后可恢复)
	FailureTypeSoft FailureType = "soft"
	// FailureTypeModelNotFound 模型不存在(需要重试但不记录 Key 熔断器)
	FailureTypeModelNotFound FailureType = "model_not_found"
	// FailureTypeNone 非故障
	FailureTypeNone FailureType = "none"
)

// FailureScope 故障归因范围（用于决定记录 Key 熔断还是模型配置熔断）
type FailureScope string

const (
	FailureScopeNone        FailureScope = "none"
	FailureScopeKey         FailureScope = "key"
	FailureScopeModelConfig FailureScope = "model_config"
	FailureScopeBoth        FailureScope = "both"
)

// FailureClassifier 故障分类器
type FailureClassifier struct{}

// NewFailureClassifier 创建故障分类器
func NewFailureClassifier() *FailureClassifier {
	return &FailureClassifier{}
}

// ClassifyHTTPError 分类 HTTP 错误
// 根据状态码和响应内容判断是硬故障还是软故障
func (fc *FailureClassifier) ClassifyHTTPError(statusCode int) (FailureType, FailureScope, string) {
	return fc.ClassifyHTTPErrorWithBody(statusCode, nil, "")
}

// ClassifyHTTPErrorWithBody 基于状态码与响应体综合分类 HTTP 错误。
//
// 401/403 本身不能直接等价为“永久 Key 失效”：它也可能来自额度、账单、
// 组织权限、区域策略等临时问题。只有响应体明确命中凭据永久失效关键词时，
// 才归类为 hard key；无法判定时默认 soft key，避免误停 Key。
func (fc *FailureClassifier) ClassifyHTTPErrorWithBody(statusCode int, body []byte, _ string) (FailureType, FailureScope, string) {
	// 2xx 成功响应
	if statusCode >= 200 && statusCode < 300 {
		return FailureTypeNone, FailureScopeNone, "正常（" + strconv.Itoa(statusCode) + "）"
	}

	bodyText := strings.ToLower(string(body))

	// 认证、授权、账单与配额问题默认归因到 Key；根据响应体再区分 hard/soft。
	if statusCode == http.StatusUnauthorized || // 401
		statusCode == http.StatusPaymentRequired || // 402
		statusCode == http.StatusForbidden { // 403
		if containsAny(bodyText, softKeyFailureKeywords) {
			return FailureTypeSoft, FailureScopeKey, "密钥额度或账单限制（" + strconv.Itoa(statusCode) + "）"
		}
		if containsAny(bodyText, hardKeyFailureKeywords) {
			return FailureTypeHard, FailureScopeKey, "密钥凭据无效（" + strconv.Itoa(statusCode) + "）"
		}
		return FailureTypeSoft, FailureScopeKey, "认证和授权问题（" + strconv.Itoa(statusCode) + "）"
	}

	// 429 需要检查是否为 quota exceeded
	if statusCode == http.StatusTooManyRequests { // 429
		return FailureTypeSoft, FailureScopeKey, "密钥余额或请求速率限制（" + strconv.Itoa(statusCode) + "）"
	}

	// 404 模型不存在，需要重试；归因到模型配置
	if statusCode == http.StatusNotFound {
		return FailureTypeModelNotFound, FailureScopeModelConfig, "上游模型不存在（" + strconv.Itoa(statusCode) + "）"
	}

	// 400 的错误一般是代理的报文结构或者内容有问题（不合适的内容），不应该熔断
	if statusCode == 400 {
		return FailureTypeSoft, FailureScopeNone, "代理客户端故障（" + strconv.Itoa(statusCode) + "）"
	}

	// 4xx 客户端错误(除了上述的)
	// 对于渠道代理来说，4xx 应该触发重试
	// 熔断相关的模型
	if statusCode > 400 && statusCode < 500 {
		return FailureTypeSoft, FailureScopeModelConfig, "代理客户端故障（" + strconv.Itoa(statusCode) + "）"
	}

	// 5xx 服务端错误,归因到模型配置（避免影响同渠道其他模型）
	if statusCode >= 500 && statusCode < 600 {
		return FailureTypeSoft, FailureScopeModelConfig, "渠道接口故障（" + strconv.Itoa(statusCode) + "）"
	}

	// 其他情况
	return FailureTypeNone, FailureScopeNone, "正常（" + strconv.Itoa(statusCode) + "）"
}

var hardKeyFailureKeywords = []string{
	"invalid api key",
	"incorrect api key",
	"api key not valid",
	"invalid key",
	"invalid token",
	"invalid bearer token",
	"authentication failed",
	"invalid authentication",
	"unauthenticated",
	"permission denied",
	"revoked api key",
	"expired api key",
}

var softKeyFailureKeywords = []string{
	"quota",
	"rate limit",
	"rate_limit",
	"too many requests",
	"billing",
	"insufficient funds",
	"insufficient credits",
	"insufficient balance",
	"balance not enough",
	"payment required",
}

func containsAny(text string, keywords []string) bool {
	if text == "" {
		return false
	}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// ClassifyNetworkError 分类网络错误
// timeout, connection refused, DNS 等错误归类为软故障
func (fc *FailureClassifier) ClassifyNetworkError(err error) (FailureType, FailureScope, string) {
	// 网络类错误归因到模型配置，避免把 Key 熔断掉
	return FailureTypeSoft, FailureScopeModelConfig, "网络异常：" + err.Error()
}

// ClassifyResponseError 综合分类响应错误
// 同时考虑 HTTP 状态码和网络错误
func (fc *FailureClassifier) ClassifyResponseError(resp *http.Response, networkErr error) (FailureType, FailureScope, string) {
	// 优先处理网络错误
	if networkErr != nil {
		return fc.ClassifyNetworkError(networkErr)
	}

	// 处理 HTTP 错误
	return fc.ClassifyHTTPError(resp.StatusCode)
}
