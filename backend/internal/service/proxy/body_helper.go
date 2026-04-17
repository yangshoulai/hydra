package proxy

import (
	"bytes"
	"io"
	"net/http"
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
func readAndCloseBody(resp *http.Response) []byte {
	if resp == nil || resp.Body == nil {
		return nil
	}
	data, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return data
}
