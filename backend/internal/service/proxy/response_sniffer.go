package proxy

import (
	"bytes"
	"errors"
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
	IsFake200     bool   // 是否为假 200
	MatchedRule   string // 匹配的规则名称
	MatchedReason string // 命中的详细原因
	Body          []byte // 响应 Body
	ContentType   string // Content-Type
	StatusCode    int    // HTTP 状态码
	IsStream      bool   // 是否流式响应
}

// Sniff 根据响应元信息与探测报文执行嗅探
// non-stream: payload 传入完整响应体
// stream: payload 传入预缓存的数据包
func (s *ResponseSniffer) Sniff(resp *http.Response, isStream bool, payload []byte, traceID string) (*SniffResult, error) {
	if resp == nil {
		return nil, errors.New("response is nil")
	}
	contentType := resp.Header.Get("Content-Type")
	result := &SniffResult{
		IsFake200:     false,
		MatchedRule:   "",
		MatchedReason: "",
		Body:          payload,
		ContentType:   contentType,
		StatusCode:    resp.StatusCode,
		IsStream:      isStream,
	}

	// 只检查 HTTP 200 响应
	if resp.StatusCode != http.StatusOK {
		return result, nil
	}

	// 应用所有嗅探规则
	for _, rule := range s.rules {
		if rule.Check(payload, contentType) {
			result.IsFake200 = true
			result.MatchedRule = rule.Name()
			result.MatchedReason = rule.Explain(payload, contentType)

			logAttrs := []any{
				"trace_id", traceID,
				"rule", result.MatchedRule,
				"reason", result.MatchedReason,
				"is_stream", isStream,
				"content_type", contentType,
				"body_preview", truncateString(string(payload), 200),
			}
			s.logger.Warn("命中假 200 嗅探规则，响应被判定为异常", logAttrs...)
			break
		}
	}

	return result, nil
}

// SniffResponse 嗅探 HTTP 响应
// 返回嗅探结果,包含是否为假 200 及原始 Body
func (s *ResponseSniffer) SniffResponse(resp *http.Response, traceID string) (*SniffResult, error) {
	// 读取 Body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("读取响应内容异常",
			"trace_id", traceID,
			"error", err,
			"status_code", resp.StatusCode,
		)
		return nil, err
	}
	// 关闭原 Body
	resp.Body.Close()

	// 重新设置 Body,使调用者可以再次读取
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	return s.Sniff(resp, false, body, traceID)
}

// IsFake200 快速检查是否为假 200
func (s *ResponseSniffer) IsFake200(resp *http.Response, traceID string) (bool, error) {
	result, err := s.SniffResponse(resp, traceID)
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
