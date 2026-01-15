package proxy

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
)

// SizeLimiter 响应大小限制器
type SizeLimiter struct {
	logger       *slog.Logger
	maxSize      int64 // 最大响应大小(字节)
	errorHandler *ErrorHandler
}

// NewSizeLimiter 创建响应大小限制器
func NewSizeLimiter(logger *slog.Logger, maxSize int64) *SizeLimiter {
	return &SizeLimiter{
		logger:       logger,
		maxSize:      maxSize,
		errorHandler: NewErrorHandler(),
	}
}

// LimitedReader 限制大小的 Reader
type LimitedReader struct {
	reader      io.Reader
	maxSize     int64
	bytesRead   int64
	sizeExceeded bool
}

// NewLimitedReader 创建限制大小的 Reader
func NewLimitedReader(reader io.Reader, maxSize int64) *LimitedReader {
	return &LimitedReader{
		reader:  reader,
		maxSize: maxSize,
	}
}

// Read 读取数据并检查大小限制
func (lr *LimitedReader) Read(p []byte) (n int, err error) {
	if lr.sizeExceeded {
		return 0, errors.New("response size limit exceeded")
	}

	n, err = lr.reader.Read(p)
	lr.bytesRead += int64(n)

	if lr.bytesRead > lr.maxSize {
		lr.sizeExceeded = true
		return n, errors.New("response size limit exceeded")
	}

	return n, err
}

// BytesRead 返回已读取的字节数
func (lr *LimitedReader) BytesRead() int64 {
	return lr.bytesRead
}

// SizeExceeded 返回是否超过大小限制
func (lr *LimitedReader) SizeExceeded() bool {
	return lr.sizeExceeded
}

// CheckResponseSize 检查响应大小
// 如果响应 Content-Length 超过限制，直接返回错误
func (sl *SizeLimiter) CheckResponseSize(resp *http.Response) error {
	if resp.ContentLength > 0 && resp.ContentLength > sl.maxSize {
		sl.logger.Warn("response size exceeds limit",
			slog.Int64("content_length", resp.ContentLength),
			slog.Int64("max_size", sl.maxSize),
		)
		return errors.New("response size exceeds limit")
	}
	return nil
}

// WrapReader 包装 Reader 以限制读取大小
func (sl *SizeLimiter) WrapReader(reader io.Reader) *LimitedReader {
	return NewLimitedReader(reader, sl.maxSize)
}

// GetMaxSize 返回最大响应大小
func (sl *SizeLimiter) GetMaxSize() int64 {
	return sl.maxSize
}
