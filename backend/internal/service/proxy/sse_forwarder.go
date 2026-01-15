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
	c.Header("X-Accel-Buffering", "no") // 禁用Nginx缓冲
	c.Header("Transfer-Encoding", "chunked") // 使用分块传输

	// 获取响应写入器
	writer := c.Writer
	flusher, ok := writer.(http.Flusher)
	if !ok {
		sf.logger.Error("response writer does not support flushing")
		return io.ErrUnexpectedEOF
	}

	// 不使用 bufio.Reader，直接读取以避免缓冲
	// 创建一个小缓冲区来逐字节/逐小块读取
	buf := make([]byte, 1) // 每次读取1字节，确保实时性
	defer upstreamResp.Body.Close()

	var lineBuffer bytes.Buffer
	bytesSent := 0
	eventCount := 0

	sf.logger.Info("starting SSE stream forwarding",
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
				sf.logger.Info("SSE stream completed",
					slog.Int("bytes_sent", bytesSent),
					slog.Int("event_count", eventCount),
				)
				break
			}
			sf.logger.Error("error reading SSE stream",
				slog.String("error", err.Error()),
				slog.Int("bytes_sent", bytesSent),
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
				sf.logger.Debug("SSE event forwarded",
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
			sf.logger.Warn("client connection closed",
				slog.Int("bytes_sent", bytesSent),
				slog.Int("event_count", eventCount),
			)
			return c.Request.Context().Err()
		default:
			// 继续处理
		}
	}

	return nil
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
