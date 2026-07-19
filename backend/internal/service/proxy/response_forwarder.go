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
	ResponseBody          string
	ResponseBodyTruncated bool
	ResponseCommitted     bool
	StreamChunks          int
	FirstChunkMS          int
}

const (
	// 长流的完整响应只用于 token 统计和调试记录；不能让它无限占用代理进程内存。
	// 使用尾部缓存可保留通常位于结束事件中的 usage 字段。
	maxStreamCaptureBytes      = 2 * 1024 * 1024
	maxSSERewritePendingBytes  = 1 * 1024 * 1024
	streamCaptureTruncatedNote = "[Hydra: 流式响应超过 2 MiB，以下仅保留末尾报文用于排障]"
)

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
		"Retry-After",
		"RateLimit-Limit",
		"RateLimit-Remaining",
		"RateLimit-Reset",
		"X-RateLimit-Limit",
		"X-RateLimit-Remaining",
		"X-RateLimit-Reset",
		"X-RateLimit-Limit-Requests",
		"X-RateLimit-Remaining-Requests",
		"X-RateLimit-Reset-Requests",
		"X-RateLimit-Limit-Tokens",
		"X-RateLimit-Remaining-Tokens",
		"X-RateLimit-Reset-Tokens",
		"OpenAI-Processing-Ms",
		"OpenAI-Organization",
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
	return isSSEContentType(trimmed)
}

func isSSEContentType(contentType string) bool {
	trimmed := strings.TrimSpace(contentType)
	if trimmed == "" {
		return false
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
	channelModel string,
	model string,
	traceID string,
	keepaliveInterval time.Duration,
	streamIdleTimeout time.Duration,
) (*StreamForwardResult, error) {
	startTime := time.Now()
	firstChunkMS := 0
	firstChunkSeen := false
	responseCommitted := false

	capture := newStreamCapture(maxStreamCaptureBytes)
	streamCounter := newStreamChunkCounter()
	modelRewriter := (*streamModelRewriter)(nil)
	// Gemini 等端点的流式响应并不保证采用 SSE；只能在明确的 SSE 响应中按 data 行改写。
	// 否则为了等换行而缓存，会把非 SSE 的长流误变成全量缓冲。
	if isSSEContentType(upstreamResp.Header.Get("Content-Type")) {
		modelRewriter = newStreamModelRewriter(channelModel, model)
	}
	buildResult := func() *StreamForwardResult {
		return &StreamForwardResult{
			ResponseBody:          capture.String(),
			ResponseBodyTruncated: capture.Truncated(),
			ResponseCommitted:     responseCommitted,
			StreamChunks:          streamCounter.Count(),
			FirstChunkMS:          firstChunkMS,
		}
	}

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

	var idleTimer *time.Timer
	var idleC <-chan time.Time
	resetIdleTimer := func() {
		if streamIdleTimeout <= 0 {
			return
		}
		if idleTimer == nil {
			idleTimer = time.NewTimer(streamIdleTimeout)
			idleC = idleTimer.C
			return
		}
		if !idleTimer.Stop() {
			select {
			case <-idleTimer.C:
			default:
			}
		}
		idleTimer.Reset(streamIdleTimeout)
		idleC = idleTimer.C
	}
	resetIdleTimer()
	defer func() {
		if idleTimer == nil {
			return
		}
		if !idleTimer.Stop() {
			select {
			case <-idleTimer.C:
			default:
			}
		}
	}()

	writePart := func(part []byte) error {
		if len(part) == 0 {
			return nil
		}
		if !firstChunkSeen {
			firstChunkSeen = true
			firstChunkMS = int(time.Since(startTime).Milliseconds())
		}

		resetKeepaliveTimer()
		if !responseCommitted {
			// 只有确认有可转发上游字节后才提交 2xx；空流和首包空闲仍可安全重试。
			c.Status(upstreamResp.StatusCode)
			responseCommitted = true
		}
		if _, err := c.Writer.Write(part); err != nil {
			return err
		}
		flusher.Flush()
		capture.Write(part)
		streamCounter.Observe(part)
		return nil
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return buildResult(), c.Request.Context().Err()
		case <-keepaliveC:
			if _, err := c.Writer.Write([]byte(": keepalive\n\n")); err != nil {
				return buildResult(), err
			}
			flusher.Flush()
			resetKeepaliveTimer()
		case <-idleC:
			// Close 会中断 net/http Body.Read，确保 reader goroutine 不会永久卡住。
			_ = upstreamResp.Body.Close()
			return buildResult(), ErrUpstreamStreamIdle
		case result := <-resultCh:
			if len(result.part) > 0 {
				resetIdleTimer()
				// 模型名改写可能要等到完整 data 行才输出，但首包耗时应按上游首字节计算。
				if !firstChunkSeen {
					firstChunkSeen = true
					firstChunkMS = int(time.Since(startTime).Milliseconds())
				}
				part := modelRewriter.Transform(result.part, false)
				if err := writePart(part); err != nil {
					return buildResult(), err
				}
			}

			if result.err == nil {
				continue
			}
			if result.err == io.EOF {
				if err := writePart(modelRewriter.Transform(nil, true)); err != nil {
					return buildResult(), err
				}
				goto streamEnd
			}
			return buildResult(), result.err
		}
	}

streamEnd:
	if capture.Len() == 0 {
		return nil, &EmptySSEBodyError{
			TraceID: traceID,
			Message: "空流式响应体",
		}
	}

	return buildResult(), nil
}

// streamCapture 保留有界的流式尾部报文。token usage 通常在末尾事件中，
// 因而比简单截取前缀更适合计费统计；调试日志会明确标识已截断。
type streamCapture struct {
	limit     int
	data      []byte
	truncated bool
}

func newStreamCapture(limit int) *streamCapture {
	if limit <= 0 {
		limit = maxStreamCaptureBytes
	}
	return &streamCapture{limit: limit}
}

func (c *streamCapture) Write(part []byte) {
	if c == nil || len(part) == 0 {
		return
	}
	if len(part) >= c.limit {
		c.data = append(c.data[:0], part[len(part)-c.limit:]...)
		c.truncated = true
		return
	}
	if len(c.data)+len(part) <= c.limit {
		c.data = append(c.data, part...)
		return
	}

	overflow := len(c.data) + len(part) - c.limit
	if cap(c.data) < c.limit {
		tail := make([]byte, c.limit)
		copy(tail, c.data[overflow:])
		copy(tail[c.limit-len(part):], part)
		c.data = tail
	} else {
		copy(c.data, c.data[overflow:])
		c.data = c.data[:c.limit-len(part)]
		c.data = append(c.data, part...)
	}
	c.truncated = true
}

func (c *streamCapture) Len() int {
	if c == nil {
		return 0
	}
	return len(c.data)
}

func (c *streamCapture) String() string {
	if c == nil {
		return ""
	}
	return string(c.data)
}

func (c *streamCapture) Truncated() bool {
	return c != nil && c.truncated
}

// streamChunkCounter 只缓存 data: 前缀状态，避免无换行异常流持续堆积内存。
type streamChunkCounter struct {
	leadingWhitespace bool
	prefix            [len("data:")]byte
	prefixLen         int
	dataLine          bool
	count             int
}

func newStreamChunkCounter() *streamChunkCounter {
	return &streamChunkCounter{leadingWhitespace: true}
}

func (c *streamChunkCounter) Observe(part []byte) {
	if c == nil {
		return
	}
	for _, ch := range part {
		if ch == '\n' {
			if c.dataLine {
				c.count++
			}
			c.leadingWhitespace = true
			c.prefixLen = 0
			c.dataLine = false
			continue
		}
		if c.leadingWhitespace {
			if ch == ' ' || ch == '\t' || ch == '\r' {
				continue
			}
			c.leadingWhitespace = false
		}
		if c.prefixLen < len(c.prefix) {
			c.prefix[c.prefixLen] = ch
			c.prefixLen++
			if c.prefixLen == len(c.prefix) {
				c.dataLine = bytes.Equal(c.prefix[:], []byte("data:"))
			}
		}
	}
}

func (c *streamChunkCounter) Count() int {
	if c == nil {
		return 0
	}
	return c.count
}

func formatTruncatedStreamCapture(body string) string {
	return streamCaptureTruncatedNote + "\n" + body
}

// streamModelRewriter 在 SSE 的完整 data 行内把渠道模型名映射回统一模型名。
// 行可能跨多个网络 chunk，因而只在拿到换行后写回，避免输出半截 JSON。
type streamModelRewriter struct {
	channelModel string
	model        string
	pending      []byte
	disabled     bool
}

func newStreamModelRewriter(channelModel, model string) *streamModelRewriter {
	return &streamModelRewriter{channelModel: channelModel, model: model}
}

func (r *streamModelRewriter) Transform(part []byte, final bool) []byte {
	if r == nil || r.disabled || r.channelModel == "" || r.model == "" || r.channelModel == r.model {
		return part
	}
	if len(r.pending)+len(part) > maxSSERewritePendingBytes {
		// 非法或极大的单行 SSE 事件不能无限缓冲；保持原报文直通并停用本请求的改写。
		output := append(append([]byte(nil), r.pending...), part...)
		r.pending = nil
		r.disabled = true
		return output
	}

	r.pending = append(r.pending, part...)
	var output bytes.Buffer
	for {
		index := bytes.IndexByte(r.pending, '\n')
		if index < 0 {
			break
		}
		line := append([]byte(nil), r.pending[:index+1]...)
		r.pending = r.pending[index+1:]
		output.Write(r.rewriteLine(line))
	}
	if final && len(r.pending) > 0 {
		output.Write(r.rewriteLine(r.pending))
		r.pending = nil
	}
	return output.Bytes()
}

func (r *streamModelRewriter) rewriteLine(line []byte) []byte {
	trimmed := bytes.TrimSpace(line)
	if !bytes.HasPrefix(trimmed, []byte("data:")) {
		return line
	}

	payload := bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("data:")))
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return line
	}

	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return line
	}
	if !rewriteModelFields(data, r.channelModel, r.model) {
		return line
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return line
	}

	lineEnding := []byte("\n")
	if bytes.HasSuffix(line, []byte("\r\n")) {
		lineEnding = []byte("\r\n")
	}
	leadingLen := len(line) - len(bytes.TrimLeft(line, " \t"))
	prefix := append([]byte(nil), line[:leadingLen]...)
	prefix = append(prefix, []byte("data: ")...)
	prefix = append(prefix, encoded...)
	prefix = append(prefix, lineEnding...)
	return prefix
}

// replaceModelInJSON 在 JSON 响应中替换模型名
func (rf *ResponseForwarder) replaceModelInJSON(body []byte, channelModel string, model string, traceID string) ([]byte, error) {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return body, err
	}

	if !rewriteModelFields(data, channelModel, model) {
		return body, nil
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

// rewriteModelFields 兼容 Chat Completions 根 model 与 Responses 事件中的 response.model。
func rewriteModelFields(value any, channelModel string, model string) bool {
	if channelModel == "" || model == "" || channelModel == model {
		return false
	}
	changed := false
	switch data := value.(type) {
	case map[string]any:
		if upstreamModel, ok := data["model"].(string); ok && upstreamModel == channelModel {
			data["model"] = model
			changed = true
		}
		for _, nested := range data {
			if rewriteModelFields(nested, channelModel, model) {
				changed = true
			}
		}
	case []any:
		for _, nested := range data {
			if rewriteModelFields(nested, channelModel, model) {
				changed = true
			}
		}
	}
	return changed
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
