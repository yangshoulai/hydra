package proxy

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/service/config"
)

// SSEForwarderWithSniffer 支持嗅探的 SSE 流式响应转发器
type SSEForwarderWithSniffer struct {
	logger           *slog.Logger
	settingService   *config.SettingService
	detectionTimeout int // 首帧嗅探超时（毫秒）
	errorKeywords    []string
	errorKeywordsMu  sync.RWMutex
}

// NewSSEForwarderWithSniffer 创建支持嗅探的 SSE 转发器
func NewSSEForwarderWithSniffer(
	logger *slog.Logger,
	settingService *config.SettingService,
	detectionTimeout int, // 首帧嗅探超时（毫秒），默认 1000ms
) *SSEForwarderWithSniffer {
	if detectionTimeout <= 0 {
		detectionTimeout = 1000 // 默认 1 秒
	}

	forwarder := &SSEForwarderWithSniffer{
		logger:           logger,
		settingService:   settingService,
		detectionTimeout: detectionTimeout,
	}

	// 初始化错误关键词
	forwarder.loadErrorKeywords()

	return forwarder
}

// FirstFrameResult 首帧检测结果
type FirstFrameResult struct {
	ContainsError bool   // 是否包含错误
	ErrorType     string // 错误类型
	FirstChunk    []byte // 首帧数据
	HasMoreData   bool   // 是否还有更多数据
}

// DetectFirstFrame 检测首帧是否包含错误
// 读取第一个 SSE 事件，检查是否包含明显的错误信息
func (sf *SSEForwarderWithSniffer) DetectFirstFrame(resp *http.Response) (*FirstFrameResult, error) {
	if resp == nil || resp.Body == nil {
		return nil, io.ErrUnexpectedEOF
	}

	result := &FirstFrameResult{
		ContainsError: false,
		FirstChunk:    []byte{},
		HasMoreData:   true,
	}

	buf := make([]byte, 1)
	var lineBuffer bytes.Buffer
	newlineCount := 0
	const maxNewlines = 5 // 只检测前5行

	for {
		n, err := resp.Body.Read(buf)
		if err != nil {
			if err == io.EOF {
				result.HasMoreData = false
				break
			}
			return nil, err
		}

		if n == 0 {
			continue
		}

		ch := buf[0]
		result.FirstChunk = append(result.FirstChunk, ch)

		// 收集所有字符到行缓冲区
		lineBuffer.WriteByte(ch)

		// 遇到换行，检查一行内容
		if ch == '\n' {
			line := strings.TrimSpace(lineBuffer.String())

			// 检查 data: 行
			if strings.HasPrefix(line, "data:") {
				dataContent := strings.TrimPrefix(line, "data:")
				dataContent = strings.TrimSpace(dataContent)

				// 检查是否为结束标记
				if dataContent == "[DONE]" {
					result.HasMoreData = false
					break
				}

				// 检查数据内容是否包含错误
				if sf.containsError(dataContent) {
					result.ContainsError = true
					result.ErrorType = "error_in_data"
					sf.logger.Warn("SSE首帧检测到错误", slog.String("data", truncateStringInSniffer(dataContent, 2048)))
					break
				}
			}

			lineBuffer.Reset()
			newlineCount++

			// 只检测前几行
			if newlineCount >= maxNewlines {
				break
			}
		}

		// 限制首帧大小（避免读取太多数据）
		if len(result.FirstChunk) >= 4096 {
			break
		}
	}

	return result, nil
}

// loadErrorKeywords 从配置服务加载错误关键词
func (sf *SSEForwarderWithSniffer) loadErrorKeywords() {
	keywords := sf.settingService.GetStreamErrorRules(context.Background())

	sf.errorKeywordsMu.Lock()
	sf.errorKeywords = keywords
	sf.errorKeywordsMu.Unlock()

	sf.logger.Info("流式错误关键词已加载", "count", len(keywords))
}

// UpdateErrorKeywords 更新错误关键词
func (sf *SSEForwarderWithSniffer) UpdateErrorKeywords(keywords []string) {
	sf.errorKeywordsMu.Lock()
	sf.errorKeywords = keywords
	sf.errorKeywordsMu.Unlock()

	sf.logger.Info("流式错误关键词已更新", "count", len(keywords))
}

// containsError 检查数据是否包含错误信息
func (sf *SSEForwarderWithSniffer) containsError(data string) bool {
	dataLower := strings.ToLower(data)

	// 从配置中读取错误关键词
	sf.errorKeywordsMu.RLock()
	errorKeywords := sf.errorKeywords
	sf.errorKeywordsMu.RUnlock()

	// 检查是否包含任何关键词
	for _, keyword := range errorKeywords {
		if strings.Contains(dataLower, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}

// ForwardStreamWithDetection 支持首帧嗅探的流式转发
func (sf *SSEForwarderWithSniffer) ForwardStreamWithDetection(
	c *gin.Context,
	upstreamResp *http.Response,
	traceID string,
) (string, int, int, error) {
	startTime := time.Now()

	// 设置响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Header("Transfer-Encoding", "chunked")

	writer := c.Writer
	flusher, ok := writer.(http.Flusher)
	if !ok {
		sf.logger.Error("response writer does not support flushing")
		return "", 0, 0, io.ErrUnexpectedEOF
	}

	// 1. 检测首帧
	firstFrame, err := sf.DetectFirstFrame(upstreamResp)
	if err != nil {
		sf.logger.Error("检测SSE首帧失败", slog.String("trace_id", traceID), slog.String("error", err.Error()))
		return "", 0, 0, err
	}

	// 计算首帧响应时间（毫秒）
	firstChunkTime := int(time.Since(startTime).Milliseconds())

	// 2. 如果首帧包含错误，返回错误
	if firstFrame.ContainsError {
		sf.logger.Warn("SSE首帧包含错误信息，中断流式响应",
			slog.String("trace_id", traceID),
			slog.String("error_type", firstFrame.ErrorType),
			slog.Int("first_frame_size", len(firstFrame.FirstChunk)),
		)
		return string(firstFrame.FirstChunk), 0, firstChunkTime, &Fake200Error{
			TraceID: traceID,
			Message: "fake 200 in SSE first frame",
			Body:    string(firstFrame.FirstChunk),
		}
	}

	// 首帧为空且没有更多数据，视为异常（用于触发重试）
	if len(firstFrame.FirstChunk) == 0 && !firstFrame.HasMoreData {
		return "", 0, firstChunkTime, &EmptySSEBodyError{
			TraceID: traceID,
			Message: "empty sse response body",
		}
	}

	// 3. 首帧正常，继续流式转发
	c.Status(http.StatusOK)
	flusher.Flush()

	// 先转发首帧数据
	if len(firstFrame.FirstChunk) > 0 {
		if _, err := writer.Write(firstFrame.FirstChunk); err != nil {
			sf.logger.Error("写入首帧到客户端失败",
				slog.String("trace_id", traceID),
				slog.String("error", err.Error()),
			)
			return string(firstFrame.FirstChunk), 0, firstChunkTime, err
		}
		flusher.Flush()
	}

	// 继续转发剩余数据
	var captureBuffer bytes.Buffer
	const maxCaptureLength = 10 * 1024 * 1024
	bytesSent := len(firstFrame.FirstChunk)
	chunkCount := 0
	var lineBuffer bytes.Buffer

	buf := make([]byte, 1)
	for firstFrame.HasMoreData {
		n, err := upstreamResp.Body.Read(buf)
		if err != nil {
			if err == io.EOF {
				sf.logger.Info("流式转发完成",
					slog.String("trace_id", traceID),
					slog.Int("bytes_sent", bytesSent),
					slog.Int("chunk_count", chunkCount),
				)
				break
			}
			return captureBuffer.String(), chunkCount, firstChunkTime, err
		}

		if n == 0 {
			continue
		}

		ch := buf[0]
		bytesSent++

		// 写入客户端
		if _, err := writer.Write(buf); err != nil {
			sf.logger.Error("写入SSE数据失败",
				slog.String("trace_id", traceID),
				slog.String("error", err.Error()),
			)
			return captureBuffer.String(), chunkCount, firstChunkTime, err
		}

		// 捕获内容
		if captureBuffer.Len() < maxCaptureLength {
			captureBuffer.WriteByte(ch)
		}

		// 统计 chunk 数量（遇到 data: 行就计数）
		lineBuffer.WriteByte(ch)
		if ch == '\n' {
			line := strings.TrimSpace(lineBuffer.String())
			if strings.HasPrefix(line, "data:") {
				chunkCount++
			}
			lineBuffer.Reset()
		}

		// 立即刷新
		flusher.Flush()

		// 检查客户端断开
		select {
		case <-c.Request.Context().Done():
			sf.logger.Warn("SSE转发期间客户端连接关闭",
				slog.String("trace_id", traceID),
				slog.Int("bytes_sent", bytesSent),
			)
			return captureBuffer.String(), chunkCount, firstChunkTime, c.Request.Context().Err()
		default:
		}
	}

	capturedContent := captureBuffer.String()
	if captureBuffer.Len() >= maxCaptureLength {
		capturedContent += "...(truncated)"
	}

	return capturedContent, chunkCount, firstChunkTime, nil
}

// truncateString 截断字符串（使用 sse_forwarder.go 中的实现）
func truncateStringInSniffer(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Fake200Error 假200错误（用于流式响应首帧检测）
type Fake200Error struct {
	TraceID string
	Message string
	Body    string
}

func (e *Fake200Error) Error() string {
	return e.Message
}

func (e *Fake200Error) Timeout() bool {
	return false
}

func (e *Fake200Error) Temporary() bool {
	return false
}

// EmptySSEBodyError 空响应体错误（用于触发重试）
type EmptySSEBodyError struct {
	TraceID string
	Message string
}

func (e *EmptySSEBodyError) Error() string {
	return e.Message
}

func (e *EmptySSEBodyError) Timeout() bool {
	return false
}

func (e *EmptySSEBodyError) Temporary() bool {
	return false
}
