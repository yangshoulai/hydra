package logger

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// DebugLogger 调试日志记录器(写入文件,记录完整 Body)
type DebugLogger struct {
	logger  *slog.Logger
	enabled bool
}

// DebugLoggerConfig 调试日志配置
type DebugLoggerConfig struct {
	Enabled    bool
	FilePath   string
	MaxSize    int  // MB
	MaxBackups int
	MaxAge     int  // days
	Compress   bool
}

// NewDebugLogger 创建调试日志记录器
func NewDebugLogger(config *DebugLoggerConfig) (*DebugLogger, error) {
	if config == nil || !config.Enabled {
		return &DebugLogger{
			logger:  slog.Default(),
			enabled: false,
		}, nil
	}

	// 确保日志目录存在
	dir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// 创建 Lumberjack 日志轮转写入器
	fileWriter := &lumberjack.Logger{
		Filename:   config.FilePath,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   config.Compress,
	}

	// 创建专用的调试日志 handler
	handler := slog.NewJSONHandler(fileWriter, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	})

	logger := slog.New(handler)

	return &DebugLogger{
		logger:  logger,
		enabled: true,
	}, nil
}

// LogRequest 记录请求详细信息
func (d *DebugLogger) LogRequest(traceID, method, url string, headers map[string]string, body string) {
	if !d.enabled {
		return
	}

	d.logger.Debug("请求详情",
		slog.String("trace_id", traceID),
		slog.String("method", method),
		slog.String("url", url),
		slog.Any("headers", headers),
		slog.String("body", d.truncateIfNeeded(body, 10000)),
	)
}

// LogResponse 记录响应详细信息
func (d *DebugLogger) LogResponse(traceID string, statusCode int, headers map[string]string, body string) {
	if !d.enabled {
		return
	}

	d.logger.Debug("响应详情",
		slog.String("trace_id", traceID),
		slog.Int("status_code", statusCode),
		slog.Any("headers", headers),
		slog.String("body", d.truncateIfNeeded(body, 10000)),
	)
}

// LogRequestBody 记录完整请求 Body（仅在出错或 Debug 模式时）
func (d *DebugLogger) LogRequestBody(traceID, body string) {
	if !d.enabled {
		return
	}

	d.logger.Debug("完整请求体",
		slog.String("trace_id", traceID),
		slog.String("body", body),
		slog.Int("body_length", len(body)),
	)
}

// LogResponseBody 记录完整响应 Body（仅在出错或 Debug 模式时）
func (d *DebugLogger) LogResponseBody(traceID, body string) {
	if !d.enabled {
		return
	}

	d.logger.Debug("完整响应体",
		slog.String("trace_id", traceID),
		slog.String("body", body),
		slog.Int("body_length", len(body)),
	)
}

// LogError 记录错误详细信息
func (d *DebugLogger) LogError(traceID, errorMsg string, errorDetails interface{}) {
	if !d.enabled {
		return
	}

	d.logger.Error("错误详情",
		slog.String("trace_id", traceID),
		slog.String("error", errorMsg),
		slog.Any("details", errorDetails),
	)
}

// LogProxyAttempt 记录代理尝试信息
func (d *DebugLogger) LogProxyAttempt(traceID string, attemptNum int, channelName string, keyID uint) {
	if !d.enabled {
		return
	}

	d.logger.Debug("代理尝试",
		slog.String("trace_id", traceID),
		slog.Int("attempt_number", attemptNum),
		slog.String("channel_name", channelName),
		slog.Uint64("key_id", uint64(keyID)),
	)
}

// LogRetry 记录重试信息
func (d *DebugLogger) LogRetry(traceID string, reason string, retryCount int) {
	if !d.enabled {
		return
	}

	d.logger.Warn("触发重试",
		slog.String("trace_id", traceID),
		slog.String("reason", reason),
		slog.Int("retry_count", retryCount),
	)
}

// LogCircuitBreaker 记录熔断器状态变化
func (d *DebugLogger) LogCircuitBreaker(traceID string, keyID uint, oldState, newState string) {
	if !d.enabled {
		return
	}

	d.logger.Warn("熔断器状态变更",
		slog.String("trace_id", traceID),
		slog.Uint64("key_id", uint64(keyID)),
		slog.String("old_state", oldState),
		slog.String("new_state", newState),
	)
}

// LogStructured 记录结构化数据（用于复杂对象）
func (d *DebugLogger) LogStructured(traceID string, message string, data interface{}) {
	if !d.enabled {
		return
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		d.logger.Error("序列化结构化数据失败",
			slog.String("trace_id", traceID),
			slog.String("error", err.Error()),
		)
		return
	}

	d.logger.Debug(message,
		slog.String("trace_id", traceID),
		slog.String("data", string(jsonData)),
	)
}

// IsEnabled 检查调试日志是否启用
func (d *DebugLogger) IsEnabled() bool {
	return d.enabled
}

// truncateIfNeeded 如果字符串过长则截断
func (d *DebugLogger) truncateIfNeeded(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return fmt.Sprintf("%s... (truncated, total length: %d)", s[:maxLen], len(s))
}
