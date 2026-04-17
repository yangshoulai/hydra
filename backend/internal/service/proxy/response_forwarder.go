package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// NonStreamForwardResult 非流式转发结果
type NonStreamForwardResult struct {
	ResponseBody string
}

// StreamForwardResult 流式转发结果
type StreamForwardResult struct {
	ResponseBody string
	StreamChunks int
	FirstChunkMS int
}

// ResponseForwarder 响应转发器（统一处理流式与非流式）
type ResponseForwarder struct {
	logger *slog.Logger
}

// NewResponseForwarder 创建响应转发器
func NewResponseForwarder(logger *slog.Logger) *ResponseForwarder {
	return &ResponseForwarder{
		logger: logger,
	}
}

// copyResponseHeaders 复制响应头
func (rf *ResponseForwarder) copyResponseHeaders(upstreamResp *http.Response, c *gin.Context) {
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

// ForwardNonStreamResponse 转发非流式响应
// 调用方负责在进入该函数前完成嗅探与响应校验。
func (rf *ResponseForwarder) ForwardNonStreamResponse(
	c *gin.Context,
	upstreamResp *http.Response,
	body []byte,
	channelModel string,
	model string,
	traceID string,
) (*NonStreamForwardResult, error) {
	contentType := upstreamResp.Header.Get("Content-Type")
	if len(body) > 0 && (contentType == "" || bytes.Contains([]byte(contentType), []byte("application/json"))) {
		modifiedBody, replaceErr := rf.replaceModelInJSON(body, channelModel, model, traceID)
		if replaceErr != nil {
			rf.logger.Debug("替换 JSON 响应中的模型名失败",
				slog.String("trace_id", traceID),
				slog.String("error", replaceErr.Error()),
			)
		} else {
			body = modifiedBody
		}
	}

	rf.copyResponseHeaders(upstreamResp, c)
	c.Status(upstreamResp.StatusCode)
	if c.Writer.Header().Get("Content-Type") == "" {
		c.Header("Content-Type", "application/json")
	}

	const chunkSize = 16 * 1024
	for i := 0; i < len(body); i += chunkSize {
		end := i + chunkSize
		if end > len(body) {
			end = len(body)
		}
		if _, err := c.Writer.Write(body[i:end]); err != nil {
			return &NonStreamForwardResult{
				ResponseBody: string(body),
			}, err
		}
		c.Writer.Flush()
	}

	return &NonStreamForwardResult{
		ResponseBody: string(body),
	}, nil
}

// ForwardStreamResponse 转发流式响应
// 调用方负责在进入该函数前完成嗅探与探测缓存回灌。
func (rf *ResponseForwarder) ForwardStreamResponse(
	c *gin.Context,
	upstreamResp *http.Response,
	traceID string,
) (*StreamForwardResult, error) {
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(upstreamResp.Body)

	startTime := time.Now()
	firstChunkMS := 0
	firstChunkSeen := false

	var (
		captureBuffer bytes.Buffer
		lineBuffer    bytes.Buffer
		streamChunks  int
	)

	rf.copyResponseHeaders(upstreamResp, c)
	if c.Writer.Header().Get("Content-Type") == "" {
		c.Header("Content-Type", "text/event-stream; charset=utf-8")
	}
	if c.Writer.Header().Get("Cache-Control") == "" {
		c.Header("Cache-Control", "no-cache")
	}
	if c.Writer.Header().Get("Connection") == "" {
		c.Header("Connection", "keep-alive")
	}
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, io.ErrUnexpectedEOF
	}

	c.Status(upstreamResp.StatusCode)
	flusher.Flush()

	chunkBuf := make([]byte, 1024)
	for {
		n, readErr := upstreamResp.Body.Read(chunkBuf)
		if n > 0 {
			if !firstChunkSeen {
				firstChunkSeen = true
				firstChunkMS = int(time.Since(startTime).Milliseconds())
			}

			part := chunkBuf[:n]
			if _, err := c.Writer.Write(part); err != nil {
				return &StreamForwardResult{
					ResponseBody: captureBuffer.String(),
					StreamChunks: streamChunks,
					FirstChunkMS: firstChunkMS,
				}, err
			}
			flusher.Flush()
			captureBuffer.Write(part)

			for i := 0; i < len(part); i++ {
				ch := part[i]
				lineBuffer.WriteByte(ch)
				if ch == '\n' {
					line := strings.TrimSpace(lineBuffer.String())
					if strings.HasPrefix(line, "data:") {
						streamChunks++
					}
					lineBuffer.Reset()
				}
			}

			select {
			case <-c.Request.Context().Done():
				return &StreamForwardResult{
					ResponseBody: captureBuffer.String(),
					StreamChunks: streamChunks,
					FirstChunkMS: firstChunkMS,
				}, c.Request.Context().Err()
			default:
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return &StreamForwardResult{
				ResponseBody: captureBuffer.String(),
				StreamChunks: streamChunks,
				FirstChunkMS: firstChunkMS,
			}, readErr
		}
	}

	if captureBuffer.Len() == 0 {
		return nil, &EmptySSEBodyError{
			TraceID: traceID,
			Message: "空流式响应体",
		}
	}

	return &StreamForwardResult{
		ResponseBody: captureBuffer.String(),
		StreamChunks: streamChunks,
		FirstChunkMS: firstChunkMS,
	}, nil
}

// replaceModelInJSON 在 JSON 响应中替换模型名
func (rf *ResponseForwarder) replaceModelInJSON(body []byte, channelModel string, model string, traceID string) ([]byte, error) {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return body, err
	}

	if upstream, ok := data["model"].(string); ok && upstream == channelModel {
		data["model"] = model
	}

	modifiedBody, err := json.Marshal(data)
	if err != nil {
		return body, err
	}

	rf.logger.Debug("响应中的模型名已替换",
		slog.String("trace_id", traceID),
		slog.String("channel_model", channelModel),
		slog.String("model", model),
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
