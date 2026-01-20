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
	endpoint string,
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

	// 根据 Content-Type 处理请求
	contentType := c.GetHeader("Content-Type")
	var modifiedBody []byte

	if strings.Contains(contentType, "application/json") {
		// 解析 JSON 并替换模型名
		var reqData map[string]interface{}
		if err := json.Unmarshal(originalBody, &reqData); err != nil {
			return nil, originalBody, err
		}

		// 替换模型名
		reqData["model"] = routeResult.UpstreamModel

		// 序列化回 JSON
		modifiedBody, err = json.Marshal(reqData)
		if err != nil {
			return nil, originalBody, err
		}
	} else {
		// 非 JSON 请求,直接使用原始 Body
		modifiedBody = originalBody
	}

	// 构建上游 URL
	upstreamURL := strings.TrimRight(routeResult.Channel.BaseURL, "/") + "/" + strings.TrimLeft(endpoint, "/")

	// 创建新请求
	req, err := http.NewRequest(c.Request.Method, upstreamURL, bytes.NewBuffer(modifiedBody))
	if err != nil {
		return nil, originalBody, err
	}

	// 复制必要的 Headers
	rb.copyHeaders(c.Request, req)

	// 设置 Content-Type
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else {
		req.Header.Set("Content-Type", "application/json")
	}

	// 使用端点的配置方法设置请求头
	ep, err := rb.getEndpointByPath(endpoint)
	if err == nil {
		ep.ConfigureRequest(req, routeResult.Key.KeyValue)
	} else {
		// 如果无法获取端点，使用默认配置
		req.Header.Set("Authorization", "Bearer "+routeResult.Key.KeyValue)
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

// copyHeaders 复制必要的请求头
func (rb *RequestBuilder) copyHeaders(src *http.Request, dst *http.Request) {
	// 需要复制的 Headers 白名单
	// 注意：不复制 Accept-Encoding，让 Go 的 HTTP 客户端自动处理 gzip 解压
	headersToCopy := []string{
		"Accept",
		"Accept-Language",
		"User-Agent",
		"X-Request-Id",
	}

	for _, header := range headersToCopy {
		if value := src.Header.Get(header); value != "" {
			dst.Header.Set(header, value)
		}
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
func (rb *RequestBuilder) IsStreamRequest(body []byte) bool {
	var reqData map[string]interface{}
	if err := json.Unmarshal(body, &reqData); err != nil {
		return false
	}

	stream, ok := reqData["stream"].(bool)
	return ok && stream
}

// GetModelFromRequest 从请求中提取模型名
func (rb *RequestBuilder) GetModelFromRequest(body []byte) (string, error) {
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
