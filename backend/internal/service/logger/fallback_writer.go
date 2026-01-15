package logger

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// FallbackWriter 带降级功能的 Writer
// 当主 Writer 写入失败时,自动降级到备用 Writer (stderr)
type FallbackWriter struct {
	primary       io.Writer     // 主 Writer (通常是文件)
	fallback      io.Writer     // 备用 Writer (stderr)
	logger        *slog.Logger  // 用于记录降级事件
	mu            sync.RWMutex
	
	// 状态管理
	useFallback   atomic.Bool   // 是否使用降级模式
	failureCount  atomic.Int64  // 连续失败次数
	lastFailTime  time.Time     // 最后一次失败时间
	
	// 配置参数
	maxFailures   int           // 最大连续失败次数,超过后启用降级
	retryInterval time.Duration // 重试间隔
	
	// 统计信息
	totalWrites      atomic.Int64 // 总写入次数
	primaryWrites    atomic.Int64 // 主 Writer 成功次数
	fallbackWrites   atomic.Int64 // 备用 Writer 成功次数
	failedWrites     atomic.Int64 // 完全失败次数
}

// FallbackWriterConfig 降级 Writer 配置
type FallbackWriterConfig struct {
	Primary       io.Writer
	Fallback      io.Writer
	MaxFailures   int
	RetryInterval time.Duration
	Logger        *slog.Logger
}

// NewFallbackWriter 创建带降级功能的 Writer
func NewFallbackWriter(cfg *FallbackWriterConfig) *FallbackWriter {
	if cfg.Fallback == nil {
		cfg.Fallback = os.Stderr
	}
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 3
	}
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = 30 * time.Second
	}

	fw := &FallbackWriter{
		primary:       cfg.Primary,
		fallback:      cfg.Fallback,
		logger:        cfg.Logger,
		maxFailures:   cfg.MaxFailures,
		retryInterval: cfg.RetryInterval,
		lastFailTime:  time.Time{},
	}

	// 初始状态不使用降级
	fw.useFallback.Store(false)

	return fw
}

// Write 实现 io.Writer 接口
func (fw *FallbackWriter) Write(p []byte) (n int, err error) {
	fw.totalWrites.Add(1)

	// 检查是否应该尝试恢复到主 Writer
	if fw.useFallback.Load() {
		fw.mu.RLock()
		shouldRetry := time.Since(fw.lastFailTime) >= fw.retryInterval
		fw.mu.RUnlock()

		if shouldRetry {
			// 尝试恢复到主 Writer
			if fw.tryRecoverToPrimary() {
				fw.logInfo("recovered to primary writer")
			}
		}
	}

	// 尝试写入主 Writer
	if !fw.useFallback.Load() {
		n, err = fw.writeToPrimary(p)
		if err == nil {
			fw.primaryWrites.Add(1)
			return n, nil
		}

		// 主 Writer 失败,记录失败
		fw.handlePrimaryFailure(err)
	}

	// 使用备用 Writer
	n, err = fw.writeToFallback(p)
	if err != nil {
		fw.failedWrites.Add(1)
		fw.logError("both primary and fallback writers failed", err)
		return n, err
	}

	fw.fallbackWrites.Add(1)
	return n, nil
}

// writeToPrimary 写入主 Writer
func (fw *FallbackWriter) writeToPrimary(p []byte) (int, error) {
	if fw.primary == nil {
		return 0, errors.New("primary writer is nil")
	}

	return fw.primary.Write(p)
}

// writeToFallback 写入备用 Writer
func (fw *FallbackWriter) writeToFallback(p []byte) (int, error) {
	if fw.fallback == nil {
		return 0, errors.New("fallback writer is nil")
	}

	return fw.fallback.Write(p)
}

// handlePrimaryFailure 处理主 Writer 失败
func (fw *FallbackWriter) handlePrimaryFailure(err error) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	fw.failureCount.Add(1)
	fw.lastFailTime = time.Now()

	currentFailures := fw.failureCount.Load()

	// 检查是否应该启用降级模式
	if !fw.useFallback.Load() && currentFailures >= int64(fw.maxFailures) {
		fw.useFallback.Store(true)
		fw.logWarn(fmt.Sprintf("switching to fallback writer after %d consecutive failures", currentFailures), err)
	} else if !fw.useFallback.Load() {
		fw.logDebug(fmt.Sprintf("primary writer failed (%d/%d)", currentFailures, fw.maxFailures), err)
	}
}

// tryRecoverToPrimary 尝试恢复到主 Writer
func (fw *FallbackWriter) tryRecoverToPrimary() bool {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// 尝试写入一小段测试数据
	testData := []byte("")
	_, err := fw.writeToPrimary(testData)
	
	if err == nil {
		// 恢复成功
		fw.useFallback.Store(false)
		fw.failureCount.Store(0)
		return true
	}

	// 恢复失败,更新最后失败时间
	fw.lastFailTime = time.Now()
	return false
}

// GetStats 获取统计信息
func (fw *FallbackWriter) GetStats() map[string]interface{} {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	return map[string]interface{}{
		"total_writes":      fw.totalWrites.Load(),
		"primary_writes":    fw.primaryWrites.Load(),
		"fallback_writes":   fw.fallbackWrites.Load(),
		"failed_writes":     fw.failedWrites.Load(),
		"using_fallback":    fw.useFallback.Load(),
		"failure_count":     fw.failureCount.Load(),
		"last_failure_time": fw.lastFailTime,
	}
}

// IsFallbackMode 检查是否处于降级模式
func (fw *FallbackWriter) IsFallbackMode() bool {
	return fw.useFallback.Load()
}

// ForceRecovery 强制尝试恢复到主 Writer
func (fw *FallbackWriter) ForceRecovery() bool {
	return fw.tryRecoverToPrimary()
}

// logInfo 记录信息日志
func (fw *FallbackWriter) logInfo(msg string) {
	if fw.logger != nil {
		fw.logger.Info(msg)
	}
}

// logWarn 记录警告日志
func (fw *FallbackWriter) logWarn(msg string, err error) {
	if fw.logger != nil {
		if err != nil {
			fw.logger.Warn(msg, slog.String("error", err.Error()))
		} else {
			fw.logger.Warn(msg)
		}
	}
}

// logError 记录错误日志
func (fw *FallbackWriter) logError(msg string, err error) {
	if fw.logger != nil {
		if err != nil {
			fw.logger.Error(msg, slog.String("error", err.Error()))
		} else {
			fw.logger.Error(msg)
		}
	}
}

// logDebug 记录调试日志
func (fw *FallbackWriter) logDebug(msg string, err error) {
	if fw.logger != nil {
		if err != nil {
			fw.logger.Debug(msg, slog.String("error", err.Error()))
		} else {
			fw.logger.Debug(msg)
		}
	}
}

// Close 关闭 Writer(如果实现了 io.Closer)
func (fw *FallbackWriter) Close() error {
	var errors []error

	// 尝试关闭主 Writer
	if closer, ok := fw.primary.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close primary writer: %w", err))
		}
	}

	// 尝试关闭备用 Writer(通常是 stderr,不需要关闭)
	if closer, ok := fw.fallback.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close fallback writer: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing writers: %v", errors)
	}

	return nil
}

// NewFallbackWriterWithLumberjack 创建带 Lumberjack 的降级 Writer
func NewFallbackWriterWithLumberjack(filePath string, maxSize, maxBackups, maxAge int, compress bool, logger *slog.Logger) *FallbackWriter {
	primary := &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   compress,
	}

	return NewFallbackWriter(&FallbackWriterConfig{
		Primary:       primary,
		Fallback:      os.Stderr,
		MaxFailures:   3,
		RetryInterval: 30 * time.Second,
		Logger:        logger,
	})
}
