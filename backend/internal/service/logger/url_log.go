package logger

import (
	"net/url"
	"regexp"
	"strings"
)

const redactedURLValue = "REDACTED"

var (
	fallbackSensitiveQueryPattern = regexp.MustCompile(`(?i)([?&](?:key|api_key|apikey|access_token|token|secret|client_secret|signature|sig|password|pwd|auth|authorization)=)[^&\s]+`)
	fallbackUserPasswordPattern   = regexp.MustCompile(`(?i)(://[^/\s:@]+:)[^@/\s]+@`)
	fallbackUserOnlyPattern       = regexp.MustCompile(`(?i)(://)[^/\s:@]+@`)
)

// SafeURLForLog 返回适合写入日志的 URL 字符串。
//
// 处理范围：
//   - query 中常见密钥字段脱敏，例如 key / api_key / access_token；
//   - userinfo 中的密码脱敏，例如 http://user:pass@proxy；
//   - 解析失败时尽量用正则兜底，避免把明显的凭证原文写入日志。
func SafeURLForLog(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return redactURLTextFallback(trimmed)
	}
	return SafeURLValueForLog(parsed)
}

// SafeURLValueForLog 返回适合写入日志的 URL 字符串，不修改传入的 URL。
func SafeURLValueForLog(value *url.URL) string {
	if value == nil {
		return ""
	}

	cloned := *value
	redactURLUserInfo(&cloned)
	redactURLQuery(&cloned)
	return cloned.String()
}

func redactURLUserInfo(value *url.URL) {
	if value == nil || value.User == nil {
		return
	}

	username := value.User.Username()
	if _, ok := value.User.Password(); ok {
		value.User = url.UserPassword(username, redactedURLValue)
		return
	}
	// 只有 userinfo 而没有密码时，常见于把 token 放在 user 位置，保守脱敏。
	value.User = url.User(redactedURLValue)
}

func redactURLQuery(value *url.URL) {
	if value == nil || value.RawQuery == "" {
		return
	}

	query := value.Query()
	changed := false
	for key := range query {
		if isSensitiveURLQueryKey(key) {
			query.Set(key, redactedURLValue)
			changed = true
		}
	}
	if changed {
		value.RawQuery = query.Encode()
	}
}

func isSensitiveURLQueryKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "key",
		"api_key",
		"apikey",
		"access_token",
		"token",
		"secret",
		"client_secret",
		"signature",
		"sig",
		"password",
		"pwd",
		"auth",
		"authorization":
		return true
	default:
		return strings.HasSuffix(normalized, "_key") ||
			strings.HasSuffix(normalized, "_token") ||
			strings.HasSuffix(normalized, "_secret")
	}
}

func redactURLTextFallback(raw string) string {
	redacted := fallbackSensitiveQueryPattern.ReplaceAllString(raw, "${1}"+redactedURLValue)
	redacted = fallbackUserPasswordPattern.ReplaceAllString(redacted, "${1}"+redactedURLValue+"@")
	redacted = fallbackUserOnlyPattern.ReplaceAllString(redacted, "${1}"+redactedURLValue+"@")
	return redacted
}
