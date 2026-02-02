package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/endpoint"
)

// RequestBuilder 代理请求构建器
type RequestBuilder struct{}

// NewRequestBuilder 创建请求构建器
func NewRequestBuilder() *RequestBuilder {
	return &RequestBuilder{}
}

// ChatCompletionRequest OpenAI Chat Completion 请求结构
type ChatCompletionRequest struct {
	Model            string                   `json:"model"`
	Messages         []map[string]interface{} `json:"messages"`
	Temperature      *float64                 `json:"temperature,omitempty"`
	TopP             *float64                 `json:"top_p,omitempty"`
	N                *int                     `json:"n,omitempty"`
	Stream           bool                     `json:"stream,omitempty"`
	Stop             interface{}              `json:"stop,omitempty"`
	MaxTokens        *int                     `json:"max_tokens,omitempty"`
	PresencePenalty  *float64                 `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64                 `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]float64       `json:"logit_bias,omitempty"`
	User             string                   `json:"user,omitempty"`
}

// BuildProxyRequest 构建代理请求
// 将客户端请求转换为上游请求,包括:
// 1. 替换模型名
// 2. 设置正确的 Headers
// 3. 构建请求 Body
func (rb *RequestBuilder) BuildProxyRequest(
	c *gin.Context,
	routeResult *RouteResult,
	ep endpoint.Endpoint,
) (*http.Request, []byte, error) {
	if routeResult == nil {
		return nil, nil, errors.New("route result is nil")
	}

	// 读取原始请求 Body
	originalBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, nil, err
	}
	// 重置 Body,使其可以再次读取
	c.Request.Body = io.NopCloser(bytes.NewBuffer(originalBody))

	modifiedBody := originalBody

	// 构建上游 URL
	requestPath := strings.TrimLeft(c.Request.URL.Path, "/")
	upstreamURL := strings.TrimRight(routeResult.Channel.BaseURL, "/") + "/" + requestPath
	if c.Request.URL.RawQuery != "" {
		upstreamURL += "?" + c.Request.URL.RawQuery
	}

	// 创建新请求（绑定客户端上下文，客户端断开时可自动取消上游请求）
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, upstreamURL, bytes.NewBuffer(modifiedBody))
	if err != nil {
		return nil, originalBody, err
	}

	// 复制必要的 Headers
	rb.copyHeaders(c.Request, req)

	// 使用端点的配置方法设置请求头和请求体
	updatedBody, err := ep.ConfigureRequest(req, routeResult.Key.KeyValue, routeResult.UpstreamModel, modifiedBody)
	if err != nil {
		return nil, originalBody, err
	}
	if updatedBody != nil {
		modifiedBody = updatedBody
		req.Body = io.NopCloser(bytes.NewBuffer(modifiedBody))
		req.ContentLength = int64(len(modifiedBody))
	}

	return req, originalBody, nil
}

// getEndpointByPath 根据路径获取端点
func (rb *RequestBuilder) getEndpointByPath(path string) (endpoint.Endpoint, error) {
	// 遍历所有端点，找到匹配的路径
	for _, ep := range endpoint.GetAll() {
		if ep.GetPath() == path {
			return ep, nil
		}
	}
	return nil, errors.New("endpoint not found")
}

// copyHeaders 尽量复制客户端请求头
// 仅过滤可能导致代理异常的 hop-by-hop 头以及会与上游鉴权冲突的头
func (rb *RequestBuilder) copyHeaders(src *http.Request, dst *http.Request) {
	hopByHop := map[string]struct{}{
		"connection":          {},
		"proxy-connection":    {},
		"keep-alive":          {},
		"te":                  {},
		"trailer":             {},
		"transfer-encoding":   {},
		"upgrade":             {},
		"proxy-authorization": {},
		"proxy-authenticate":  {},
	}

	// 连接头里声明的 hop-by-hop 头也要过滤
	for _, value := range src.Header.Values("Connection") {
		for _, item := range strings.Split(value, ",") {
			name := strings.ToLower(strings.TrimSpace(item))
			if name != "" {
				hopByHop[name] = struct{}{}
			}
		}
	}

	for key, values := range src.Header {
		lowerKey := strings.ToLower(key)
		if _, skip := hopByHop[lowerKey]; skip {
			continue
		}
		// 不转发 Authorization，避免覆盖上游 Key
		if lowerKey == "authorization" {
			continue
		}
		// 不转发 Host/Content-Length，交由 http.Client 自行处理
		if lowerKey == "host" || lowerKey == "content-length" {
			continue
		}
		// 不转发 Accept-Encoding，避免上游返回压缩响应导致解析异常
		if lowerKey == "accept-encoding" {
			continue
		}
		dst.Header[key] = append([]string(nil), values...)
	}
}

// ParseChatCompletionRequest 解析 Chat Completion 请求
func (rb *RequestBuilder) ParseChatCompletionRequest(body []byte) (*ChatCompletionRequest, error) {
	var req ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// IsStreamRequest 判断是否为流式请求
func (rb *RequestBuilder) IsStreamRequest(body []byte, endpointType string, requestPath string) bool {
	if endpointType == "gemini" {
		return strings.Contains(requestPath, ":streamGenerateContent")
	}

	var reqData map[string]interface{}
	if err := json.Unmarshal(body, &reqData); err != nil {
		return false
	}

	stream, ok := reqData["stream"].(bool)
	return ok && stream
}

// GetModelFromRequest 从请求中提取模型名
func (rb *RequestBuilder) GetModelFromRequest(body []byte, endpointType string, requestPath string) (string, error) {
	if endpointType == "gemini" {
		if model, err := rb.getModelFromGeminiPath(requestPath); err == nil {
			return model, nil
		}
		return rb.getModelFromRequestBody(body)
	}

	return rb.getModelFromRequestBody(body)
}

func (rb *RequestBuilder) getModelFromRequestBody(body []byte) (string, error) {
	var reqData map[string]interface{}
	if err := json.Unmarshal(body, &reqData); err != nil {
		return "", err
	}

	model, ok := reqData["model"].(string)
	if !ok || model == "" {
		return "", errors.New("model field is missing or invalid")
	}

	return model, nil
}

func (rb *RequestBuilder) getModelFromGeminiPath(requestPath string) (string, error) {
	index := strings.Index(requestPath, "/models/")
	if index < 0 {
		return "", errors.New("model field is missing or invalid")
	}

	raw := requestPath[index+len("/models/"):]
	if raw == "" {
		return "", errors.New("model field is missing or invalid")
	}

	parts := strings.SplitN(raw, ":", 2)
	model := parts[0]
	if model == "" {
		return "", errors.New("model field is missing or invalid")
	}

	return model, nil
}
