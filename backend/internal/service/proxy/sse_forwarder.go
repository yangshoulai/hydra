package proxy

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SSEForwarder SSE 流式响应转发器
type SSEForwarder struct {
	logger *slog.Logger
}

// NewSSEForwarder 创建 SSE 转发器
func NewSSEForwarder(logger *slog.Logger) *SSEForwarder {
	return &SSEForwarder{
		logger: logger,
	}
}

// ForwardStream 转发 SSE 流式响应
// 从上游响应读取 SSE 事件并转发到客户端
func (sf *SSEForwarder) ForwardStream(c *gin.Context, upstreamResp *http.Response) error {
	// 设置响应头 - 必须在写入任何数据之前设置
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")      // 禁用Nginx缓冲
	c.Header("Transfer-Encoding", "chunked") // 使用分块传输

	// 获取响应写入器
	writer := c.Writer
	flusher, ok := writer.(http.Flusher)
	if !ok {
		sf.logger.Error("响应流不支持 flushing")
		return io.ErrUnexpectedEOF
	}

	// 不使用 bufio.Reader，直接读取以避免缓冲
	// 创建一个小缓冲区来逐字节/逐小块读取
	buf := make([]byte, 1) // 每次读取1字节，确保实时性
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(upstreamResp.Body)

	var lineBuffer bytes.Buffer
	bytesSent := 0
	eventCount := 0

	sf.logger.Info("开始SSE流转发",
		slog.String("content_type", upstreamResp.Header.Get("Content-Type")),
		slog.String("content_encoding", upstreamResp.Header.Get("Content-Encoding")),
	)

	// 写入HTTP状态码
	c.Status(http.StatusOK)

	// 立即刷新一次，确保头部发送
	flusher.Flush()

	for {
		// 每次读取1字节
		n, err := upstreamResp.Body.Read(buf)
		if err != nil {
			if err == io.EOF {
				// 流结束
				sf.logger.Info("SSE流正常完成",
					slog.Int("bytes_sent", bytesSent),
					slog.Int("event_count", eventCount),
				)
				break
			}
			// 上游读取错误
			sf.logger.Error("error reading from upstream SSE stream",
				slog.String("error", err.Error()),
				slog.String("error_type", err.Error()),
				slog.Int("bytes_sent", bytesSent),
				slog.Int("event_count", eventCount),
				slog.Int("upstream_status", upstreamResp.StatusCode),
			)
			return err
		}

		if n == 0 {
			continue
		}

		ch := buf[0]
		bytesSent++

		// 写入客户端
		if _, err := writer.Write(buf); err != nil {
			sf.logger.Error("error writing to client",
				slog.String("error", err.Error()),
				slog.Int("bytes_sent", bytesSent),
				slog.Int("event_count", eventCount),
				slog.String("client_ip", c.ClientIP()),
			)
			return err
		}

		// 追加到行缓冲
		lineBuffer.WriteByte(ch)

		// 检测是否是行结束
		if ch == '\n' {
			// 检查是否是空行（事件结束）
			line := lineBuffer.String()
			if len(line) > 2 { // \r\n 或 \n
				eventCount++
				sf.logger.Debug("转发 SSE 流",
					slog.Int("event_number", eventCount),
					slog.Int("line_length", len(line)),
					slog.String("line_prefix", truncateString(line, 50)),
				)
			}
			lineBuffer.Reset()
		}

		// 每次写入后立即刷新 - 关键！
		// 这样可以确保每个字节都立即发送给客户端
		flusher.Flush()

		// 检查客户端是否断开连接
		select {
		case <-c.Request.Context().Done():
			contextErr := c.Request.Context().Err()
			sf.logger.Warn("SSE转发期间客户端连接关闭",
				slog.Int("bytes_sent", bytesSent),
				slog.Int("event_count", eventCount),
				slog.String("context_error", contextErr.Error()),
				slog.String("client_ip", c.ClientIP()),
			)
			return contextErr
		default:
			// 继续处理
		}
	}

	return nil
}

// ForwardStreamWithCapture 转发 SSE 流式响应并捕获内容用于日志记录
// 返回捕获的响应内容字符串
func (sf *SSEForwarder) ForwardStreamWithCapture(c *gin.Context, upstreamResp *http.Response, traceID string) (string, error) {
	// 设置响应头 - 必须在写入任何数据之前设置
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")      // 禁用Nginx缓冲
	c.Header("Transfer-Encoding", "chunked") // 使用分块传输

	// 获取响应写入器
	writer := c.Writer
	flusher, ok := writer.(http.Flusher)
	if !ok {
		sf.logger.Error("response writer does not support flushing")
		return "", io.ErrUnexpectedEOF
	}

	// 不使用 bufio.Reader，直接读取以避免缓冲
	// 创建一个小缓冲区来逐字节/逐小块读取
	buf := make([]byte, 1) // 每次读取1字节，确保实时性
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(upstreamResp.Body)

	var lineBuffer bytes.Buffer
	var captureBuffer bytes.Buffer // 用于捕获内容
	bytesSent := 0
	eventCount := 0

	// 最大捕获长度（10MB）
	const maxCaptureLength = 10 * 1024 * 1024

	sf.logger.Info("开始SSE流转发并捕获内容",
		slog.String("trace_id", traceID),
		slog.String("content_type", upstreamResp.Header.Get("Content-Type")),
		slog.String("content_encoding", upstreamResp.Header.Get("Content-Encoding")),
	)

	// 写入HTTP状态码
	c.Status(http.StatusOK)

	// 立即刷新一次，确保头部发送
	flusher.Flush()

	for {
		// 每次读取1字节
		n, err := upstreamResp.Body.Read(buf)
		if err != nil {
			if err == io.EOF {
				// 流结束
				sf.logger.Info("SSE流正常完成",
					slog.String("trace_id", traceID),
					slog.Int("bytes_sent", bytesSent),
					slog.Int("event_count", eventCount),
				)
				break
			}
			// 上游读取错误
			sf.logger.Error("error reading from upstream SSE stream",
				slog.String("trace_id", traceID),
				slog.String("error", err.Error()),
				slog.String("error_type", err.Error()),
				slog.Int("bytes_sent", bytesSent),
				slog.Int("event_count", eventCount),
				slog.Int("upstream_status", upstreamResp.StatusCode),
			)
			return captureBuffer.String(), err
		}

		if n == 0 {
			continue
		}

		ch := buf[0]
		bytesSent++

		// 写入客户端
		if _, err := writer.Write(buf); err != nil {
			sf.logger.Error("error writing to client",
				slog.String("trace_id", traceID),
				slog.String("error", err.Error()),
				slog.Int("bytes_sent", bytesSent),
				slog.Int("event_count", eventCount),
				slog.String("client_ip", c.ClientIP()),
			)
			return captureBuffer.String(), err
		}

		// 追加到行缓冲
		lineBuffer.WriteByte(ch)

		// 如果还没超过最大长度，也追加到捕获缓冲
		if captureBuffer.Len() < maxCaptureLength {
			captureBuffer.WriteByte(ch)
		}

		// 检测是否是行结束
		if ch == '\n' {
			// 检查是否是空行（事件结束）
			line := lineBuffer.String()
			if len(line) > 2 { // \r\n 或 \n
				eventCount++
				sf.logger.Debug("转发 SSE 流已完成",
					slog.String("trace_id", traceID),
					slog.Int("event_number", eventCount),
					slog.Int("line_length", len(line)),
					slog.String("line_prefix", truncateString(line, 50)),
				)
			}
			lineBuffer.Reset()
		}

		// 每次写入后立即刷新 - 关键！
		// 这样可以确保每个字节都立即发送给客户端
		flusher.Flush()

		// 检查客户端是否断开连接
		select {
		case <-c.Request.Context().Done():
			contextErr := c.Request.Context().Err()
			sf.logger.Warn("SSE转发期间客户端连接关闭",
				slog.String("trace_id", traceID),
				slog.Int("bytes_sent", bytesSent),
				slog.Int("event_count", eventCount),
				slog.String("context_error", contextErr.Error()),
				slog.String("client_ip", c.ClientIP()),
			)
			return captureBuffer.String(), contextErr
		default:
			// 继续处理
		}
	}

	capturedContent := captureBuffer.String()
	if captureBuffer.Len() >= maxCaptureLength {
		capturedContent += "...(truncated)"
	}

	return capturedContent, nil
}

// truncateString 截断字符串用于日志显示
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ParseSSEEvent 解析 SSE 事件
func (sf *SSEForwarder) ParseSSEEvent(eventData string) map[string]string {
	event := make(map[string]string)
	lines := strings.Split(eventData, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// SSE 格式: field: value
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		field := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		event[field] = value
	}

	return event
}

// IsStreamDone 检查是否为流结束标记
func (sf *SSEForwarder) IsStreamDone(line string) bool {
	// OpenAI SSE 流结束标记
	return strings.Contains(line, "data: [DONE]")
}
