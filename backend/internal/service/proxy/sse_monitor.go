package proxy

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"time"
)

// SSEMonitor SSE 流监控器
type SSEMonitor struct {
	logger *slog.Logger
}

// NewSSEMonitor 创建 SSE 监控器
func NewSSEMonitor(logger *slog.Logger) *SSEMonitor {
	return &SSEMonitor{
		logger: logger,
	}
}

// StreamStats SSE 流统计信息
type StreamStats struct {
	BytesRead      int64
	LinesRead      int
	Duration       time.Duration
	Disconnected   bool
	DisconnectType string // client_disconnect, upstream_disconnect, read_error, write_error
	Error          error
}

// MonitoredReader 监控的 Reader
type MonitoredReader struct {
	reader       *bufio.Reader
	bytesRead    int64
	linesRead    int
	lastReadTime time.Time
	disconnected bool
	disconnectType string
	err          error
}

// NewMonitoredReader 创建监控的 Reader
func NewMonitoredReader(reader io.Reader) *MonitoredReader {
	return &MonitoredReader{
		reader:       bufio.NewReader(reader),
		lastReadTime: time.Now(),
	}
}

// ReadLine 读取一行并更新统计
func (mr *MonitoredReader) ReadLine() ([]byte, error) {
	line, err := mr.reader.ReadBytes('\n')
	mr.lastReadTime = time.Now()

	if err != nil {
		if err == io.EOF {
			mr.disconnected = true
			mr.disconnectType = "upstream_disconnect"
		} else {
			mr.disconnected = true
			mr.disconnectType = "read_error"
			mr.err = err
		}
		return line, err
	}

	mr.bytesRead += int64(len(line))
	mr.linesRead++

	return line, nil
}

// GetStats 获取统计信息
func (mr *MonitoredReader) GetStats() (int64, int, bool, string, error) {
	return mr.bytesRead, mr.linesRead, mr.disconnected, mr.disconnectType, mr.err
}

// LastReadTime 返回最后读取时间
func (mr *MonitoredReader) LastReadTime() time.Time {
	return mr.lastReadTime
}

// MonitorStream 监控 SSE 流式传输
func (m *SSEMonitor) MonitorStream(
	reader io.Reader,
	writer io.Writer,
	traceID string,
) *StreamStats {
	startTime := time.Now()
	monitoredReader := NewMonitoredReader(reader)

	stats := &StreamStats{}

	// 逐行读取并转发
	for {
		line, err := monitoredReader.ReadLine()

		if len(line) > 0 {
			// 尝试写入客户端
			_, writeErr := writer.Write(line)
			if writeErr != nil {
				// 客户端断开连接
				stats.Disconnected = true
				stats.DisconnectType = "client_disconnect"
				stats.Error = writeErr

				m.logger.Warn("client disconnected during SSE stream",
					slog.String("trace_id", traceID),
					slog.Int64("bytes_written", stats.BytesRead),
					slog.Int("lines_written", stats.LinesRead),
					slog.String("error", writeErr.Error()),
				)
				break
			}

			// Flush 确保数据发送
			if flusher, ok := writer.(interface{ Flush() }); ok {
				flusher.Flush()
			}
		}

		if err != nil {
			if err == io.EOF {
				// 正常结束
				m.logger.Info("SSE stream completed",
					slog.String("trace_id", traceID),
					slog.Int64("bytes_read", stats.BytesRead),
					slog.Int("lines_read", stats.LinesRead),
				)
			} else {
				// 上游断开或读取错误
				stats.Disconnected = true
				stats.DisconnectType = "upstream_error"
				stats.Error = err

				m.logger.Error("upstream error during SSE stream",
					slog.String("trace_id", traceID),
					slog.Int64("bytes_read", stats.BytesRead),
					slog.Int("lines_read", stats.LinesRead),
					slog.String("error", err.Error()),
				)
			}
			break
		}
	}

	// 获取最终统计
	bytesRead, linesRead, disconnected, disconnectType, err := monitoredReader.GetStats()
	stats.BytesRead = bytesRead
	stats.LinesRead = linesRead
	stats.Duration = time.Since(startTime)

	if disconnected && stats.DisconnectType == "" {
		stats.Disconnected = disconnected
		stats.DisconnectType = disconnectType
		stats.Error = err
	}

	return stats
}

// DetectStall 检测流式传输是否卡住
// timeout: 超时时间，如果在这个时间内没有读取到数据，则认为卡住
func (m *SSEMonitor) DetectStall(lastReadTime time.Time, timeout time.Duration) error {
	if time.Since(lastReadTime) > timeout {
		return errors.New("SSE stream stalled: no data received within timeout")
	}
	return nil
}

// LogStreamMetrics 记录流式传输指标
func (m *SSEMonitor) LogStreamMetrics(traceID string, stats *StreamStats) {
	m.logger.Info("SSE stream metrics",
		slog.String("trace_id", traceID),
		slog.Int64("bytes_read", stats.BytesRead),
		slog.Int("lines_read", stats.LinesRead),
		slog.Duration("duration", stats.Duration),
		slog.Bool("disconnected", stats.Disconnected),
		slog.String("disconnect_type", stats.DisconnectType),
	)
}
