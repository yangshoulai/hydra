package proxy

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

// multiReadCloser 组合前置缓存数据与剩余 body，保留关闭底层连接的能力。
// 避免使用 io.NopCloser 包裹后丢失 Close 语义导致连接无法归还连接池。
type multiReadCloser struct {
	reader io.Reader
	closer io.Closer
}

func newMultiReadCloser(prefix []byte, rest io.ReadCloser) *multiReadCloser {
	return &multiReadCloser{
		reader: io.MultiReader(bytes.NewReader(prefix), rest),
		closer: rest,
	}
}

func (m *multiReadCloser) Read(p []byte) (int, error) { return m.reader.Read(p) }

func (m *multiReadCloser) Close() error {
	if m.closer == nil {
		return nil
	}
	return m.closer.Close()
}

// drainAndCloseBody 丢弃剩余响应体并关闭，帮助底层连接归还 keep-alive 池。
const drainLimit = 64 * 1024

func drainAndCloseBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.CopyN(io.Discard, resp.Body, drainLimit)
	_ = resp.Body.Close()
}

// readAndCloseBody 读取完整响应体并关闭。用于需要采集上游失败响应 body 的调试路径。
// 仅在调用方明确想要 body（如调试模式）时使用；其余错误路径应用 drainAndCloseBody。
func readAndCloseBody(resp *http.Response, maxBytes int64) []byte {
	if resp == nil || resp.Body == nil {
		return nil
	}
	data, _ := readResponseBody(resp.Body, maxBytes)
	_ = resp.Body.Close()
	return data
}

// readResponseBody 读取非流式上游响应，并在达到配置上限时立即失败。
// 调试采集也复用该限制，避免错误渠道通过超大错误页耗尽内存。
func readResponseBody(body io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return io.ReadAll(body)
	}
	data, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrUpstreamResponseTooLarge
	}
	return data, nil
}

// readStreamChunkWithIdleTimeout 给嗅探阶段的单次读取加空闲上限。
// channel 有缓冲；超时时关闭 body 后，迟到的 Read 返回不会阻塞。
func readStreamChunkWithIdleTimeout(body io.ReadCloser, buffer []byte, timeout time.Duration) (int, error) {
	if timeout <= 0 {
		return body.Read(buffer)
	}
	type result struct {
		n   int
		err error
	}
	resultCh := make(chan result, 1)
	go func() {
		n, err := body.Read(buffer)
		resultCh <- result{n: n, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case readResult := <-resultCh:
		return readResult.n, readResult.err
	case <-timer.C:
		_ = body.Close()
		return 0, ErrUpstreamStreamIdle
	}
}
