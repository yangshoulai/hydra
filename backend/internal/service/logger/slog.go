package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	runtimeLevelVar  = &slog.LevelVar{}
	runtimeAddSource atomic.Bool
)

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level          string
	EnableFile     bool
	FilePath       string
	MaxSize        int // MB
	MaxBackups     int
	MaxAge         int // days
	Compress       bool
	EnableDatabase bool
	AddSource      bool
}

// InitLogger 初始化 Slog 结构化日志
func InitLogger(cfg *LoggerConfig) (*slog.Logger, error) {
	SetLogLevel(cfg.Level)
	SetAddSource(cfg.AddSource)

	// AddSource 固定打开；真正的开关由 dynamicSourceHandler 在运行时清 record.PC 实现
	opts := &slog.HandlerOptions{
		Level:     runtimeLevelVar,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				localTime := a.Value.Time().Local()
				a.Value = slog.StringValue(localTime.Format("2006-01-02T15:04:05Z07:00"))
			}
			return a
		},
	}

	var writers []io.Writer
	writers = append(writers, os.Stderr)

	if cfg.EnableFile && cfg.FilePath != "" {
		fileWriter := &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}
		writers = append(writers, fileWriter)
	}

	multiWriter := io.MultiWriter(writers...)

	inner := slog.NewTextHandler(multiWriter, opts)
	handler := &dynamicSourceHandler{inner: inner}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger, nil
}

// SetLogLevel 动态更新运行时日志级别
func SetLogLevel(level string) {
	runtimeLevelVar.Set(parseLogLevel(level))
}

// SetAddSource 动态切换是否在日志中输出源码位置
func SetAddSource(enabled bool) {
	runtimeAddSource.Store(enabled)
}

// dynamicSourceHandler 包装底层 handler，根据 runtimeAddSource 决定是否保留 record.PC。
// slog 内部仅在 record.PC != 0 时解析源码位置，所以把 PC 清零即可动态关闭 AddSource。
type dynamicSourceHandler struct {
	inner slog.Handler
}

func (h *dynamicSourceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *dynamicSourceHandler) Handle(ctx context.Context, r slog.Record) error {
	if !runtimeAddSource.Load() {
		r.PC = 0
	}
	return h.inner.Handle(ctx, r)
}

func (h *dynamicSourceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &dynamicSourceHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *dynamicSourceHandler) WithGroup(name string) slog.Handler {
	return &dynamicSourceHandler{inner: h.inner.WithGroup(name)}
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
