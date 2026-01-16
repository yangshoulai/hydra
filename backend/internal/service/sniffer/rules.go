package sniffer

import (
	"encoding/json"
	"regexp"
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
}

// JSONErrorRule JSON 响应中包含 error 字段的规则
type JSONErrorRule struct{}

func (r *JSONErrorRule) Name() string {
	return "JSONErrorRule"
}

func (r *JSONErrorRule) Check(body []byte, contentType string) bool {
	// 只检查 JSON 类型的响应
	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		return false
	}

	// 尝试解析为 JSON
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return false
	}

	// 检查是否包含 error 字段
	if _, hasError := data["error"]; hasError {
		return true
	}

	return false
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
	bodyStr := string(body)

	// 如果 Content-Type 是 text/html,直接判定为假 200
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}

	// 检查响应 Body 是否匹配 HTML 特征
	for _, pattern := range r.htmlPatterns {
		if pattern.MatchString(bodyStr) {
			return true
		}
	}

	return false
}

// PlainTextErrorRule 明文错误消息规则
type PlainTextErrorRule struct {
	mu            sync.RWMutex
	errorKeywords []string
}

func NewPlainTextErrorRule() *PlainTextErrorRule {
	return &PlainTextErrorRule{
		errorKeywords: GetDefaultPlainTextErrorKeywords(),
	}
}

// GetDefaultPlainTextErrorKeywords 获取默认的错误关键词
func GetDefaultPlainTextErrorKeywords() []string {
	return []string{
		"无可用后端",
		"额度不足",
		"maintenance",
		"service unavailable",
		"bad gateway",
		"gateway timeout",
		"quota exceeded",
		"rate limit",
		"unauthorized",
		"forbidden",
		"not found",
		"invalid api key",
		"invalid key",
		"authentication failed",
		"insufficient funds",
		"insufficient quota",
		"billing issue",
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
	bodyLower := strings.ToLower(string(body))

	r.mu.RLock()
	keywords := r.errorKeywords
	r.mu.RUnlock()

	// 检查是否包含任何错误关键词
	for _, keyword := range keywords {
		if strings.Contains(bodyLower, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
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

// GetDefaultRules 获取默认嗅探规则集
func GetDefaultRules() []SniffRule {
	return []SniffRule{
		&JSONErrorRule{},
		NewHTMLResponseRule(),
		NewPlainTextErrorRule(),
		&EmptyBodyRule{},
	}
}
