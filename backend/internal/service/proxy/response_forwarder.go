package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ResponseForwarder 非流式响应转发器
type ResponseForwarder struct {
	logger *slog.Logger
}

// NewResponseForwarder 创建响应转发器
func NewResponseForwarder(logger *slog.Logger) *ResponseForwarder {
	return &ResponseForwarder{
		logger: logger,
	}
}

// ForwardResponse 转发非流式响应
// 读取完整响应并转发到客户端
func (rf *ResponseForwarder) ForwardResponse(c *gin.Context, upstreamResp *http.Response, traceID string) ([]byte, error) {
	defer upstreamResp.Body.Close()

	// 读取响应 Body
	body, err := io.ReadAll(upstreamResp.Body)
	if err != nil {
		rf.logger.Error("读取上游响应体失败",
			slog.String("trace_id", traceID),
			slog.String("error", err.Error()),
			slog.Int("status_code", upstreamResp.StatusCode),
		)
		return nil, err
	}

	rf.logger.Debug("收到上游响应",
		slog.String("trace_id", traceID),
		slog.Int("status_code", upstreamResp.StatusCode),
		slog.Int("body_size", len(body)),
	)

	// 复制必要的响应头
	rf.copyResponseHeaders(upstreamResp, c)

	// 设置状态码
	c.Status(upstreamResp.StatusCode)

	// 写入响应 Body
	if len(body) > 0 {
		contentType := upstreamResp.Header.Get("Content-Type")
		if contentType != "" {
			c.Header("Content-Type", contentType)
		}

		if _, err := c.Writer.Write(body); err != nil {
			rf.logger.Error("写入响应到客户端失败",
				slog.String("trace_id", traceID),
				slog.String("error", err.Error()),
			)
			return body, err
		}
	}

	return body, nil
}

// copyResponseHeaders 复制响应头
func (rf *ResponseForwarder) copyResponseHeaders(upstreamResp *http.Response, c *gin.Context) {
	// 需要复制的 Headers 白名单
	headersToCopy := []string{
		"Content-Type",
		"Content-Encoding",
		"Cache-Control",
		"ETag",
		"Last-Modified",
		"X-Request-Id",
	}

	for _, header := range headersToCopy {
		if value := upstreamResp.Header.Get(header); value != "" {
			c.Header(header, value)
		}
	}
}

// ForwardJSONResponse 转发 JSON 响应(带模型名替换)
// 将上游模型名替换回统一模型名
// 返回响应body字符串用于日志记录
func (rf *ResponseForwarder) ForwardJSONResponse(
	c *gin.Context,
	upstreamResp *http.Response,
	upstreamModel string,
	unifiedModel string,
	traceID string,
) (string, error) {
	defer upstreamResp.Body.Close()

	// 读取响应 Body
	body, err := io.ReadAll(upstreamResp.Body)
	if err != nil {
		return "", err
	}

	// 如果是 JSON 响应,尝试替换模型名
	contentType := upstreamResp.Header.Get("Content-Type")
	if len(body) > 0 && (contentType == "" || bytes.Contains([]byte(contentType), []byte("application/json"))) {
		modifiedBody, err := rf.replaceModelInJSON(body, upstreamModel, unifiedModel, traceID)
		if err != nil {
			rf.logger.Debug("替换 JSON 响应中的模型名失败",
				slog.String("trace_id", traceID),
				slog.String("error", err.Error()),
			)
			// 失败时使用原始 Body
		} else {
			body = modifiedBody
		}
	}

	// 复制响应头
	rf.copyResponseHeaders(upstreamResp, c)

	// 设置状态码和 Content-Type
	c.Status(upstreamResp.StatusCode)
	c.Header("Content-Type", "application/json")

	// 写入响应
	if _, err := c.Writer.Write(body); err != nil {
		return string(body), err
	}

	return string(body), nil
}

// replaceModelInJSON 在 JSON 响应中替换模型名
func (rf *ResponseForwarder) replaceModelInJSON(body []byte, upstreamModel string, unifiedModel string, traceID string) ([]byte, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return body, err
	}

	// 替换顶层的 model 字段
	if model, ok := data["model"].(string); ok && model == upstreamModel {
		data["model"] = unifiedModel
	}

	// 序列化回 JSON
	modifiedBody, err := json.Marshal(data)
	if err != nil {
		return body, err
	}

	rf.logger.Debug("响应中的模型名已替换",
		slog.String("trace_id", traceID),
		slog.String("upstream_model", upstreamModel),
		slog.String("unified_model", unifiedModel),
	)

	return modifiedBody, nil
}

// ForwardErrorResponse 转发错误响应
func (rf *ResponseForwarder) ForwardErrorResponse(c *gin.Context, statusCode int, message string, traceID string) {
	rf.logger.Debug("转发错误响应",
		slog.String("trace_id", traceID),
		slog.Int("status_code", statusCode),
		slog.String("message", message),
	)

	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": message,
			"type":    "hydra_error",
			"code":    statusCode,
		},
	})
}

// GetResponseSize 获取响应大小
func (rf *ResponseForwarder) GetResponseSize(body []byte) int {
	return len(body)
}
