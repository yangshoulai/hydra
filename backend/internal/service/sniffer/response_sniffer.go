package sniffer

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
)

// ResponseSniffer 响应嗅探器,用于识别假 200 响应
type ResponseSniffer struct {
	rules  []SniffRule
	logger *slog.Logger
}

// NewResponseSniffer 创建响应嗅探器
func NewResponseSniffer(logger *slog.Logger) *ResponseSniffer {
	return &ResponseSniffer{
		rules:  GetDefaultRules(),
		logger: logger,
	}
}

// SniffResult 嗅探结果
type SniffResult struct {
	IsFake200   bool   // 是否为假 200
	MatchedRule string // 匹配的规则名称
	Body        []byte // 响应 Body
	ContentType string // Content-Type
	StatusCode  int    // HTTP 状态码
}

// SniffResponse 嗅探 HTTP 响应
// 返回嗅探结果,包含是否为假 200 及原始 Body
func (s *ResponseSniffer) SniffResponse(resp *http.Response) (*SniffResult, error) {
	// 读取 Body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read response body",
			"error", err,
			"status_code", resp.StatusCode,
		)
		return nil, err
	}
	// 关闭原 Body
	resp.Body.Close()

	// 重新设置 Body,使调用者可以再次读取
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	// 获取 Content-Type
	contentType := resp.Header.Get("Content-Type")

	result := &SniffResult{
		IsFake200:   false,
		MatchedRule: "",
		Body:        body,
		ContentType: contentType,
		StatusCode:  resp.StatusCode,
	}

	// 只检查 HTTP 200 响应
	if resp.StatusCode != http.StatusOK {
		return result, nil
	}

	// 应用所有嗅探规则
	for _, rule := range s.rules {
		if rule.Check(body, contentType) {
			result.IsFake200 = true
			result.MatchedRule = rule.Name()

			s.logger.Warn("Fake 200 response detected",
				"rule", rule.Name(),
				"content_type", contentType,
				"body_preview", truncateString(string(body), 200),
			)
			break
		}
	}

	return result, nil
}

// IsFake200 快速检查是否为假 200
func (s *ResponseSniffer) IsFake200(resp *http.Response) (bool, error) {
	result, err := s.SniffResponse(resp)
	if err != nil {
		return false, err
	}
	return result.IsFake200, nil
}

// UpdatePlainTextErrorKeywords 更新明文错误关键词
func (s *ResponseSniffer) UpdatePlainTextErrorKeywords(keywords []string) {
	for _, rule := range s.rules {
		if plainTextRule, ok := rule.(*PlainTextErrorRule); ok {
			plainTextRule.UpdateKeywords(keywords)
			return
		}
	}
}

// truncateString 截断字符串用于日志输出
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
