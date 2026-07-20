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
	runtimeLogFormat atomic.Value // string: text/json
)

const (
	LogFormatText = "text"
	LogFormatJSON = "json"
)

func init() {
	runtimeLogFormat.Store(LogFormatText)
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level          string
	Format         string
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
	SetLogFormat(cfg.Format)
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

	handler := &runtimeHandler{
		text: slog.NewTextHandler(multiWriter, opts),
		json: slog.NewJSONHandler(multiWriter, opts),
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger, nil
}

// SetLogLevel 动态更新运行时日志级别
func SetLogLevel(level string) {
	runtimeLevelVar.Set(parseLogLevel(level))
}

// SetLogFormat 动态更新日志输出格式。
func SetLogFormat(format string) {
	runtimeLogFormat.Store(normalizeLogFormat(format))
}

// SetAddSource 动态切换是否在日志中输出源码位置
func SetAddSource(enabled bool) {
	runtimeAddSource.Store(enabled)
}

// runtimeHandler 包装 text/json handler：
//  1. 根据 runtimeLogFormat 动态选择 text/json；
//  2. 根据 runtimeAddSource 决定是否保留 record.PC。
//
// slog 内部仅在 record.PC != 0 时解析源码位置，所以把 PC 清零即可动态关闭 AddSource。
type runtimeHandler struct {
	text slog.Handler
	json slog.Handler
}

func (h *runtimeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.selected().Enabled(ctx, level)
}

func (h *runtimeHandler) Handle(ctx context.Context, r slog.Record) error {
	if !runtimeAddSource.Load() {
		r.PC = 0
	}
	return h.selected().Handle(ctx, r)
}

func (h *runtimeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &runtimeHandler{text: h.text.WithAttrs(attrs), json: h.json.WithAttrs(attrs)}
}

func (h *runtimeHandler) WithGroup(name string) slog.Handler {
	return &runtimeHandler{text: h.text.WithGroup(name), json: h.json.WithGroup(name)}
}

func (h *runtimeHandler) selected() slog.Handler {
	if current, ok := runtimeLogFormat.Load().(string); ok && current == LogFormatJSON {
		return h.json
	}
	return h.text
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

func normalizeLogFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case LogFormatJSON:
		return LogFormatJSON
	default:
		return LogFormatText
	}
}
