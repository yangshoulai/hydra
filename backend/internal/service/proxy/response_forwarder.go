package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
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

func shouldSendSSEKeepalive(contentType string) bool {
	trimmed := strings.TrimSpace(contentType)
	if trimmed == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(trimmed)
	if err != nil {
		mediaType = strings.TrimSpace(strings.Split(trimmed, ";")[0])
	}
	return strings.EqualFold(mediaType, "text/event-stream")
}

func (rf *ResponseForwarder) prepareNonStreamBody(
	upstreamResp *http.Response,
	body []byte,
	channelModel string,
	model string,
	traceID string,
) []byte {
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
	return body
}

func (rf *ResponseForwarder) writeBodyChunks(c *gin.Context, body []byte) error {
	const chunkSize = 16 * 1024
	for i := 0; i < len(body); i += chunkSize {
		end := i + chunkSize
		if end > len(body) {
			end = len(body)
		}
		if _, err := c.Writer.Write(body[i:end]); err != nil {
			return err
		}
		c.Writer.Flush()
	}
	return nil
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
	body = rf.prepareNonStreamBody(upstreamResp, body, channelModel, model, traceID)

	rf.copyResponseHeaders(upstreamResp, c)
	c.Status(upstreamResp.StatusCode)
	if c.Writer.Header().Get("Content-Type") == "" {
		c.Header("Content-Type", "application/json")
	}

	if err := rf.writeBodyChunks(c, body); err != nil {
		return &NonStreamForwardResult{
			ResponseBody: string(body),
		}, err
	}

	return &NonStreamForwardResult{
		ResponseBody: string(body),
	}, nil
}

// ForwardLockedNonStreamBody 在响应头/状态码已被非流式保活提交后写入最终 JSON body。
// 调用方必须保证 body 本身仍是合法 JSON；此前写出的保活内容只能是 JSON whitespace。
func (rf *ResponseForwarder) ForwardLockedNonStreamBody(
	c *gin.Context,
	body []byte,
) (*NonStreamForwardResult, error) {
	if err := rf.writeBodyChunks(c, body); err != nil {
		return &NonStreamForwardResult{
			ResponseBody: string(body),
		}, err
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
	keepaliveInterval time.Duration,
) (*StreamForwardResult, error) {
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
	enableSSEKeepalive := shouldSendSSEKeepalive(c.Writer.Header().Get("Content-Type"))

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, io.ErrUnexpectedEOF
	}

	c.Status(upstreamResp.StatusCode)
	flusher.Flush()

	type streamReadResult struct {
		part []byte
		err  error
	}

	resultCh := make(chan streamReadResult)
	stopCh := make(chan struct{})
	defer close(stopCh)
	defer func() {
		_ = upstreamResp.Body.Close()
	}()

	go func(body io.ReadCloser) {
		chunkBuf := make([]byte, 1024)
		for {
			n, readErr := body.Read(chunkBuf)
			if n > 0 {
				part := append([]byte(nil), chunkBuf[:n]...)
				select {
				case resultCh <- streamReadResult{part: part}:
				case <-stopCh:
					return
				}
			}

			if readErr != nil {
				select {
				case resultCh <- streamReadResult{err: readErr}:
				case <-stopCh:
				}
				return
			}
		}
	}(upstreamResp.Body)

	var keepaliveTimer *time.Timer
	var keepaliveC <-chan time.Time
	resetKeepaliveTimer := func() {
		// 保活从首个上游 chunk 到达后才开始计时：在确认有可转发数据之前，
		// 不主动向客户端注入额外字节，避免过早提交响应。
		// 仅对 SSE 开启保活。NDJSON / JSONL / stream+json 也会走流式转发，
		// 但注入 SSE 注释帧会破坏这些协议的载荷格式。
		if keepaliveInterval <= 0 || !enableSSEKeepalive {
			return
		}
		if keepaliveTimer == nil {
			keepaliveTimer = time.NewTimer(keepaliveInterval)
			keepaliveC = keepaliveTimer.C
			return
		}
		if !keepaliveTimer.Stop() {
			select {
			case <-keepaliveTimer.C:
			default:
			}
		}
		keepaliveTimer.Reset(keepaliveInterval)
		keepaliveC = keepaliveTimer.C
	}
	defer func() {
		if keepaliveTimer == nil {
			return
		}
		if !keepaliveTimer.Stop() {
			select {
			case <-keepaliveTimer.C:
			default:
			}
		}
	}()

	for {
		select {
		case <-c.Request.Context().Done():
			return &StreamForwardResult{
				ResponseBody: captureBuffer.String(),
				StreamChunks: streamChunks,
				FirstChunkMS: firstChunkMS,
			}, c.Request.Context().Err()
		case <-keepaliveC:
			if _, err := c.Writer.Write([]byte(": keepalive\n\n")); err != nil {
				return &StreamForwardResult{
					ResponseBody: captureBuffer.String(),
					StreamChunks: streamChunks,
					FirstChunkMS: firstChunkMS,
				}, err
			}
			flusher.Flush()
			resetKeepaliveTimer()
		case result := <-resultCh:
			if len(result.part) > 0 {
				if !firstChunkSeen {
					firstChunkSeen = true
					firstChunkMS = int(time.Since(startTime).Milliseconds())
				}

				part := result.part
				resetKeepaliveTimer()

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
			}

			if result.err == nil {
				continue
			}
			if result.err == io.EOF {
				goto streamEnd
			}
			return &StreamForwardResult{
				ResponseBody: captureBuffer.String(),
				StreamChunks: streamChunks,
				FirstChunkMS: firstChunkMS,
			}, result.err
		}
	}

streamEnd:
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
	if !ShouldSuppressProxyLogging(c) {
		rf.logger.Debug("转发错误响应",
			slog.String("trace_id", traceID),
			slog.Int("status_code", statusCode),
			slog.String("message", message),
		)
	}

	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": message,
			"type":    "hydra_error",
			"code":    statusCode,
		},
	})
}

// ForwardLockedErrorBody 在非流式保活已提交响应后追加错误 JSON。
// 此时 HTTP 状态码已经锁定，不能再调用 c.JSON 或改写状态码。
func (rf *ResponseForwarder) ForwardLockedErrorBody(c *gin.Context, message string, traceID string) (string, error) {
	if !ShouldSuppressProxyLogging(c) {
		rf.logger.Debug("转发状态码锁定后的错误响应体",
			slog.String("trace_id", traceID),
			slog.String("message", message),
		)
	}

	payload, err := json.Marshal(gin.H{
		"error": gin.H{
			"message":       message,
			"type":          "hydra_error",
			"code":          http.StatusOK,
			"status_locked": true,
		},
	})
	if err != nil {
		return "", err
	}

	if err := rf.writeBodyChunks(c, payload); err != nil {
		return string(payload), err
	}
	return string(payload), nil
}
