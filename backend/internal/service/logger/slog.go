package logger

import (
	"io"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
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
	// 解析日志级别
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// 创建日志选项
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
		// 使用 ReplaceAttr 将时间转换为本地时间
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				// 将 UTC 时间转换为本地时间
				localTime := a.Value.Time().Local()
				a.Value = slog.StringValue(localTime.Format("2006-01-02T15:04:05Z07:00"))
			}
			return a
		},
	}

	// 构建日志输出目标
	var writers []io.Writer

	// 始终输出到 stderr(用于容器环境)
	writers = append(writers, os.Stderr)

	// 如果启用文件日志
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

	// 创建多写入器
	multiWriter := io.MultiWriter(writers...)

	// 创建 Text Handler
	handler := slog.NewTextHandler(multiWriter, opts)

	// 创建 Logger
	logger := slog.New(handler)

	// 设置为全局默认 logger
	slog.SetDefault(logger)

	return logger, nil
}

// GetLogger 获取当前全局 logger
func GetLogger() *slog.Logger {
	return slog.Default()
}
