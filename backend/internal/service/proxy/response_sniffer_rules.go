package proxy

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// SniffRule 嗅探规则接口
type SniffRule interface {
	// Check 检查响应是否为假 200
	// 返回 true 表示是假 200,false 表示正常响应
	Check(body []byte, contentType string) bool

	// Name 返回规则名称
	Name() string

	// Explain 返回命中规则的详细原因
	Explain(body []byte, contentType string) string
}

// JSONErrorRule JSON 响应中包含 error 字段的规则
type JSONErrorRule struct{}

func (r *JSONErrorRule) Name() string {
	return "JSONErrorRule"
}

func (r *JSONErrorRule) Check(body []byte, contentType string) bool {
	_, _, matched := r.detectJSONError(body, contentType)
	return matched
}

func (r *JSONErrorRule) Explain(body []byte, contentType string) string {
	source, errPreview, matched := r.detectJSONError(body, contentType)
	if !matched {
		return "未命中 JSON 错误规则"
	}
	if errPreview == "" {
		return fmt.Sprintf("%s 中包含顶层 error 字段", source)
	}
	return fmt.Sprintf("%s 中包含顶层 error 字段，error=%s", source, errPreview)
}

func (r *JSONErrorRule) detectJSONError(body []byte, contentType string) (string, string, bool) {
	normalizedType := normalizeContentType(contentType)
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "", "", false
	}

	if looksLikeJSONContentType(normalizedType) || looksLikeJSONObject(trimmed) {
		errorPreview, ok := extractTopLevelErrorPreview([]byte(trimmed))
		if ok {
			return "JSON 响应", errorPreview, true
		}
	}

	if looksLikeEventStreamContentType(normalizedType) {
		payloads := extractSSEDataPayloads(body)
		for _, payload := range payloads {
			trimmedPayload := strings.TrimSpace(payload)
			if trimmedPayload == "" || trimmedPayload == "[DONE]" || !looksLikeJSONObject(trimmedPayload) {
				continue
			}
			errorPreview, ok := extractTopLevelErrorPreview([]byte(trimmedPayload))
			if ok {
				return "SSE data 事件", errorPreview, true
			}
		}
	}

	return "", "", false
}

// HTMLResponseRule HTML 响应规则
type HTMLResponseRule struct {
	htmlPatterns []*regexp.Regexp
}

func NewHTMLResponseRule() *HTMLResponseRule {
	return &HTMLResponseRule{
		htmlPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)<!DOCTYPE\s+html`),
			regexp.MustCompile(`(?i)<html[>\s]`),
			regexp.MustCompile(`(?i)<head[>\s]`),
			regexp.MustCompile(`(?i)<body[>\s]`),
		},
	}
}

func (r *HTMLResponseRule) Name() string {
	return "HTMLResponseRule"
}

func (r *HTMLResponseRule) Check(body []byte, contentType string) bool {
	_, matched := r.detectHTMLReason(body, contentType)
	return matched
}

func (r *HTMLResponseRule) Explain(body []byte, contentType string) string {
	reason, matched := r.detectHTMLReason(body, contentType)
	if !matched {
		return "未命中 HTML 响应规则"
	}
	return reason
}

func (r *HTMLResponseRule) detectHTMLReason(body []byte, contentType string) (string, bool) {
	bodyStr := string(body)
	normalizedType := normalizeContentType(contentType)

	// 如果 Content-Type 是 text/html,直接判定为假 200
	if strings.Contains(normalizedType, "text/html") {
		return fmt.Sprintf("Content-Type=%s，响应看起来是 HTML 页面", normalizedType), true
	}

	// 检查响应 Body 是否匹配 HTML 特征
	for _, pattern := range r.htmlPatterns {
		if pattern.MatchString(bodyStr) {
			return fmt.Sprintf("响应体匹配 HTML 特征片段 %q", pattern.String()), true
		}
	}

	return "", false
}

// PlainTextErrorRule 明文错误消息规则
type PlainTextErrorRule struct {
	mu                sync.RWMutex
	errorKeywords     []string
	errorLeadPatterns []*regexp.Regexp
}

func NewPlainTextErrorRule() *PlainTextErrorRule {
	return &PlainTextErrorRule{
		errorLeadPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^(error|failed|failure|upstream|proxy|http|status|code)\s*[:：\-]?\s*`),
			regexp.MustCompile(`(?i)^(bad gateway|gateway timeout|service unavailable|quota exceeded|rate limit|unauthorized|forbidden|not found)\b`),
			regexp.MustCompile(`^(请求失败|上游|代理|渠道|鉴权|额度|限流|未授权|禁止访问|未找到|服务不可用|网关超时)\s*[:：\-]?\s*`),
		},
	}
}

// UpdateKeywords 更新错误关键词
func (r *PlainTextErrorRule) UpdateKeywords(keywords []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errorKeywords = keywords
}

func (r *PlainTextErrorRule) Name() string {
	return "PlainTextErrorRule"
}

func (r *PlainTextErrorRule) Check(body []byte, contentType string) bool {
	_, matched := r.analyze(body, contentType)
	return matched
}

func (r *PlainTextErrorRule) Explain(body []byte, contentType string) string {
	match, matched := r.analyze(body, contentType)
	if !matched {
		return "未命中明文错误规则"
	}

	reasons := make([]string, 0, 4)
	reasons = append(reasons, fmt.Sprintf("Content-Type=%s", match.ContentType))
	if len(match.MatchedKeywords) > 0 {
		reasons = append(reasons, fmt.Sprintf("命中关键词=%v", match.MatchedKeywords))
	}
	if match.HasErrorLead {
		reasons = append(reasons, "文本前缀符合错误响应格式")
	}
	if match.IsCompact {
		reasons = append(reasons, "响应足够短小")
	}
	reasons = append(reasons, fmt.Sprintf("line_count=%d", match.LineCount))
	reasons = append(reasons, fmt.Sprintf("body_length=%d", match.BodyLength))
	return "明文短错误响应：" + strings.Join(reasons, "，")
}

type plainTextRuleMatch struct {
	ContentType     string
	MatchedKeywords []string
	BodyLength      int
	LineCount       int
	FirstKeywordPos int
	HasErrorLead    bool
	IsCompact       bool
}

func (r *PlainTextErrorRule) analyze(body []byte, contentType string) (plainTextRuleMatch, bool) {
	match := plainTextRuleMatch{
		ContentType: normalizeContentType(contentType),
	}
	if looksLikeEventStreamContentType(match.ContentType) {
		// SSE 渠道有时不返回标准 JSON error，而是 `data: unauthorized` 一类明文。
		// 逐个 data payload 复用严格的短文本规则，避免把整个 SSE 帧格式当成业务内容。
		for _, payload := range extractSSEDataPayloads(body) {
			payloadMatch, matched := r.analyze([]byte(payload), "text/plain")
			if matched {
				payloadMatch.ContentType = match.ContentType
				return payloadMatch, true
			}
		}
		return match, false
	}
	if !isPlainTextCandidate(match.ContentType) {
		return match, false
	}

	trimmedBody := strings.TrimSpace(string(body))
	normalizedBody := normalizeWhitespace(trimmedBody)
	if normalizedBody == "" || looksLikeStructuredPayload(trimmedBody) {
		return match, false
	}

	match.BodyLength = len([]rune(normalizedBody))
	if match.BodyLength > 256 {
		return match, false
	}

	lines := compactLines(strings.Split(trimmedBody, "\n"))
	match.LineCount = len(lines)
	if match.LineCount == 0 || match.LineCount > 3 {
		return match, false
	}

	bodyLower := strings.ToLower(normalizedBody)
	r.mu.RLock()
	keywords := slices.Clone(r.errorKeywords)
	patterns := slices.Clone(r.errorLeadPatterns)
	r.mu.RUnlock()

	match.MatchedKeywords = make([]string, 0, len(keywords))
	match.FirstKeywordPos = -1
	for _, keyword := range keywords {
		keywordLower := strings.ToLower(strings.TrimSpace(keyword))
		if keywordLower == "" {
			continue
		}
		if idx := strings.Index(bodyLower, keywordLower); idx >= 0 {
			match.MatchedKeywords = append(match.MatchedKeywords, keyword)
			if match.FirstKeywordPos < 0 || idx < match.FirstKeywordPos {
				match.FirstKeywordPos = idx
			}
		}
	}
	if len(match.MatchedKeywords) == 0 {
		return match, false
	}

	for _, pattern := range patterns {
		if pattern.MatchString(normalizedBody) {
			match.HasErrorLead = true
			break
		}
	}

	match.IsCompact = match.BodyLength <= 96 && match.LineCount <= 2
	firstKeywordNearStart := match.FirstKeywordPos >= 0 && match.FirstKeywordPos <= 24
	if !match.HasErrorLead && len(match.MatchedKeywords) < 2 && !(match.IsCompact && firstKeywordNearStart) {
		return match, false
	}

	return match, true
}

// EmptyBodyRule 空 Body 规则(某些错误响应可能返回空 Body)
type EmptyBodyRule struct{}

func (r *EmptyBodyRule) Name() string {
	return "EmptyBodyRule"
}

func (r *EmptyBodyRule) Check(body []byte, contentType string) bool {
	// 如果 Body 为空或只包含空白字符,判定为异常
	trimmed := strings.TrimSpace(string(body))
	return len(trimmed) == 0
}

func (r *EmptyBodyRule) Explain(body []byte, contentType string) string {
	return "响应体为空或仅包含空白字符"
}

// GetDefaultRules 获取默认嗅探规则集
func GetDefaultRules() []SniffRule {
	return []SniffRule{
		&JSONErrorRule{},
		NewHTMLResponseRule(),
		NewPlainTextErrorRule(),
		&EmptyBodyRule{},
	}
}

func normalizeContentType(contentType string) string {
	normalized := strings.ToLower(strings.TrimSpace(contentType))
	if idx := strings.Index(normalized, ";"); idx >= 0 {
		normalized = strings.TrimSpace(normalized[:idx])
	}
	return normalized
}

func looksLikeJSONContentType(contentType string) bool {
	if contentType == "" {
		return false
	}
	return strings.Contains(contentType, "application/json") || strings.HasSuffix(contentType, "+json")
}

func looksLikeEventStreamContentType(contentType string) bool {
	return strings.Contains(contentType, "text/event-stream")
}

func looksLikeJSONObject(body string) bool {
	body = strings.TrimSpace(body)
	return strings.HasPrefix(body, "{") && strings.HasSuffix(body, "}")
}

func extractTopLevelErrorPreview(body []byte) (string, bool) {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return "", false
	}

	errorField, hasError := data["error"]
	if !hasError || errorField == nil {
		return "", false
	}

	switch value := errorField.(type) {
	case string:
		return truncatePreview(value, 120), true
	default:
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return "顶层 error 字段存在，但无法序列化预览", true
		}
		return truncatePreview(string(jsonBytes), 120), true
	}
}

func extractSSEDataPayloads(body []byte) []string {
	lines := strings.Split(string(body), "\n")
	payloads := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if payload == "" {
			continue
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func isPlainTextCandidate(contentType string) bool {
	if contentType == "" {
		return true
	}
	if looksLikeJSONContentType(contentType) || looksLikeEventStreamContentType(contentType) || strings.Contains(contentType, "text/html") {
		return false
	}
	return strings.HasPrefix(contentType, "text/")
}

func looksLikeStructuredPayload(body string) bool {
	body = strings.TrimSpace(body)
	if body == "" {
		return false
	}
	return looksLikeJSONObject(body) || strings.HasPrefix(body, "[") || strings.HasPrefix(body, "data:") || strings.HasPrefix(body, "event:")
}

func compactLines(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		normalized := normalizeWhitespace(strings.TrimSpace(line))
		if normalized == "" {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

func normalizeWhitespace(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func truncatePreview(input string, maxLen int) string {
	runes := []rune(strings.TrimSpace(input))
	if len(runes) <= maxLen {
		return string(runes)
	}
	return string(runes[:maxLen]) + "..."
}
