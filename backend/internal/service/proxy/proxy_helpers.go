package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const suppressProxyLoggingContextKey = "suppress_proxy_logging"

// snapshotRequestBody 读出 req.Body 并用同样内容重新包装回去，返回快照字节
func snapshotRequestBody(req *http.Request) ([]byte, *http.Request, error) {
	if req.Body == nil {
		return nil, req, nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, req, err
	}
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	return body, req, nil
}

// truncateErrorMessage 截断错误消息到 DB 字段长度内
func truncateErrorMessage(s string) string {
	const maxErrMsg = 500
	if len(s) <= maxErrMsg {
		return s
	}
	return s[:maxErrMsg]
}

// GetTraceIDFromContext 从上下文获取 TraceID
func GetTraceIDFromContext(c *gin.Context) string {
	if traceID, exists := c.Get("trace_id"); exists {
		return traceID.(string)
	}
	return ""
}

// MarkProxyLoggingSuppressed 标记当前请求应跳过代理层汇总日志与请求日志写入。
func MarkProxyLoggingSuppressed(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(suppressProxyLoggingContextKey, true)
}

// ShouldSuppressProxyLogging 判断当前请求是否应跳过代理层日志输出。
func ShouldSuppressProxyLogging(c *gin.Context) bool {
	if c == nil {
		return false
	}
	value, exists := c.Get(suppressProxyLoggingContextKey)
	if !exists {
		return false
	}
	suppressed, ok := value.(bool)
	return ok && suppressed
}

func getAccessTokenIDFromContext(c *gin.Context) uint {
	if tokenID, exists := c.Get("access_token_id"); exists {
		if id, ok := tokenID.(uint); ok {
			return id
		}
	}
	return 0
}

func getAccessTokenNameFromContext(c *gin.Context) string {
	if name, exists := c.Get("access_token_name"); exists {
		if s, ok := name.(string); ok {
			return s
		}
	}
	return ""
}

// isModelAllowedForToken 校验当前访问令牌是否有权使用该模型。
// fail-close：鉴权中间件未运行、键类型异常等情况一律拒绝。
// 仅 allowed_models 为空数组时代表"无限制"（参见 middleware/auth.go 约定）。
func isModelAllowedForToken(c *gin.Context, model string) (bool, string) {
	if _, ok := c.Get("access_token_id"); !ok {
		return false, "missing token identity"
	}

	value, exists := c.Get("access_token_allowed_models")
	if !exists {
		return false, "token permission not initialised"
	}

	allowedModels, ok := value.([]string)
	if !ok {
		return false, "token permission malformed"
	}

	if len(allowedModels) == 0 {
		return true, ""
	}

	for _, item := range allowedModels {
		if item == model {
			return true, ""
		}
	}

	return false, "model not permitted by token"
}

// isEventStreamResponse 根据 Content-Type 识别常见的流式媒体类型
func isEventStreamResponse(resp *http.Response) bool {
	raw := resp.Header.Get("Content-Type")
	if raw == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		mediaType = strings.TrimSpace(strings.Split(raw, ";")[0])
	}
	switch strings.ToLower(mediaType) {
	case "text/event-stream",
		"application/stream+json",
		"application/x-ndjson",
		"application/ndjson",
		"application/jsonl":
		return true
	}
	return false
}

// isClientCancelled 判断错误是否由客户端主动断开导致。
// client.Timeout 会表现为 DeadlineExceeded，不在此范畴。
func isClientCancelled(c *gin.Context, err error) bool {
	if c != nil && c.Request != nil && c.Request.Context().Err() == context.Canceled {
		return true
	}
	return err != nil && errors.Is(err, context.Canceled)
}

func normalizeProxyTraceID(traceID string) string {
	value := strings.TrimSpace(traceID)
	if value == "" {
		return "-"
	}
	return value
}

func normalizeStreamSniffPacketCount(count int) int {
	if count <= 0 {
		return 1
	}
	return count
}

// maskSecret 脱敏密钥字符串用于日志
func maskSecret(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "-"
	}
	if len(raw) <= 8 {
		return "****"
	}
	if len(raw) <= 12 {
		return raw[:2] + "****" + raw[len(raw)-2:]
	}
	return raw[:6] + "****" + raw[len(raw)-4:]
}

// failureTypeString / failureScopeString 在 None 时返回空串
func failureTypeString(t FailureType) string {
	if t == FailureTypeNone {
		return ""
	}
	return string(t)
}

func failureScopeString(s FailureScope) string {
	if s == FailureScopeNone {
		return ""
	}
	return string(s)
}

// sanitizeHeaders 对敏感头脱敏，返回可直接 JSON 序列化的映射
func sanitizeHeaders(h http.Header) map[string][]string {
	const mask = "***"
	if h == nil {
		return nil
	}
	out := make(map[string][]string, len(h))
	for k, v := range h {
		switch strings.ToLower(k) {
		case "authorization", "x-api-key", "x-goog-api-key", "cookie", "set-cookie":
			out[k] = []string{mask}
		default:
			out[k] = append([]string(nil), v...)
		}
	}
	return out
}

// marshalHeaders 将脱敏后的头映射序列化为 JSON，序列化失败时退化为空串
func marshalHeaders(h map[string][]string) string {
	if len(h) == 0 {
		return ""
	}
	b, err := json.Marshal(h)
	if err != nil {
		return ""
	}
	return string(b)
}

// newRouteSnapshot 把 RouteResult 转换成 ProxyContext 可长期持有的快照
func newRouteSnapshot(routeResult *RouteResult, endpointType string) *ProxyRouteSnapshot {
	return &ProxyRouteSnapshot{
		ChannelID:     routeResult.Channel.ID,
		ChannelName:   routeResult.Channel.Name,
		KeyID:         routeResult.Key.ID,
		KeyMasked:     maskSecret(routeResult.Key.ChannelKeyValue),
		ModelConfigID: routeResult.ModelConfigID,
		ChannelModel:  routeResult.ChannelModel,
		Model:         routeResult.Model,
		EndpointType:  endpointType,
	}
}
