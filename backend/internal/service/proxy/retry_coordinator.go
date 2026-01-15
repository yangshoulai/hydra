package proxy

import (
	"context"
	"log/slog"
	"time"
)

// RetryCoordinator 重试协调器,控制最大重试次数
type RetryCoordinator struct {
	logger          *slog.Logger
	maxRetries      int           // 最大重试次数
	retryDelay      time.Duration // 重试延迟
	failureClassifier *FailureClassifier
}

// NewRetryCoordinator 创建重试协调器
func NewRetryCoordinator(logger *slog.Logger, maxRetries int, retryDelay time.Duration) *RetryCoordinator {
	return &RetryCoordinator{
		logger:            logger,
		maxRetries:        maxRetries,
		retryDelay:        retryDelay,
		failureClassifier: NewFailureClassifier(),
	}
}

// RetryContext 重试上下文
type RetryContext struct {
	AttemptCount       int      // 尝试次数
	FailedChannelIDs   []uint   // 已失败的 Channel ID
	LastError          error    // 最后一次错误
	LastFailureType    FailureType // 最后一次故障类型
	StartTime          time.Time // 开始时间
}

// NewRetryContext 创建重试上下文
func NewRetryContext() *RetryContext {
	return &RetryContext{
		AttemptCount:     0,
		FailedChannelIDs: make([]uint, 0),
		StartTime:        time.Now(),
	}
}

// ShouldRetry 判断是否应该继续重试
func (rc *RetryCoordinator) ShouldRetry(retryCtx *RetryContext) bool {
	if retryCtx == nil {
		return false
	}

	// 检查是否超过最大重试次数
	if retryCtx.AttemptCount >= rc.maxRetries {
		rc.logger.Warn("max retries exceeded",
			slog.Int("attempt_count", retryCtx.AttemptCount),
			slog.Int("max_retries", rc.maxRetries),
		)
		return false
	}

	// 检查故障类型
	if !rc.failureClassifier.ShouldRetry(retryCtx.LastFailureType) {
		rc.logger.Warn("failure type does not allow retry",
			slog.String("failure_type", string(retryCtx.LastFailureType)),
		)
		return false
	}

	return true
}

// RecordAttempt 记录一次尝试
func (rc *RetryCoordinator) RecordAttempt(retryCtx *RetryContext, channelID uint, err error, failureType FailureType) {
	if retryCtx == nil {
		return
	}

	retryCtx.AttemptCount++
	retryCtx.LastError = err
	retryCtx.LastFailureType = failureType

	// 记录失败的 Channel
	if !rc.containsChannel(retryCtx.FailedChannelIDs, channelID) {
		retryCtx.FailedChannelIDs = append(retryCtx.FailedChannelIDs, channelID)
	}

	rc.logger.Info("retry attempt recorded",
		slog.Int("attempt_count", retryCtx.AttemptCount),
		slog.Uint64("channel_id", uint64(channelID)),
		slog.String("failure_type", string(failureType)),
		slog.Int("failed_channels_count", len(retryCtx.FailedChannelIDs)),
	)
}

// WaitBeforeRetry 在重试前等待
func (rc *RetryCoordinator) WaitBeforeRetry(ctx context.Context, retryCtx *RetryContext) error {
	if rc.retryDelay <= 0 {
		return nil
	}

	// 使用指数退避策略
	delay := rc.calculateDelay(retryCtx.AttemptCount)

	rc.logger.Debug("waiting before retry",
		slog.Int("attempt_count", retryCtx.AttemptCount),
		slog.Duration("delay", delay),
	)

	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// calculateDelay 计算延迟时间(指数退避)
func (rc *RetryCoordinator) calculateDelay(attemptCount int) time.Duration {
	// 指数退避: delay * (2 ^ attemptCount)
	multiplier := 1 << uint(attemptCount) // 2^attemptCount
	if multiplier > 8 {
		multiplier = 8 // 最多延迟 8 倍
	}
	return rc.retryDelay * time.Duration(multiplier)
}

// containsChannel 检查 Channel ID 是否已存在
func (rc *RetryCoordinator) containsChannel(channelIDs []uint, channelID uint) bool {
	for _, id := range channelIDs {
		if id == channelID {
			return true
		}
	}
	return false
}

// GetFailedChannels 获取已失败的 Channel ID 列表
func (rc *RetryCoordinator) GetFailedChannels(retryCtx *RetryContext) []uint {
	if retryCtx == nil {
		return []uint{}
	}
	return retryCtx.FailedChannelIDs
}

// GetAttemptCount 获取尝试次数
func (rc *RetryCoordinator) GetAttemptCount(retryCtx *RetryContext) int {
	if retryCtx == nil {
		return 0
	}
	return retryCtx.AttemptCount
}

// GetTotalDuration 获取总耗时
func (rc *RetryCoordinator) GetTotalDuration(retryCtx *RetryContext) time.Duration {
	if retryCtx == nil {
		return 0
	}
	return time.Since(retryCtx.StartTime)
}

// IsFirstAttempt 判断是否为第一次尝试
func (rc *RetryCoordinator) IsFirstAttempt(retryCtx *RetryContext) bool {
	return retryCtx != nil && retryCtx.AttemptCount == 0
}

// HasMoreRetries 判断是否还有重试机会
func (rc *RetryCoordinator) HasMoreRetries(retryCtx *RetryContext) bool {
	if retryCtx == nil {
		return false
	}
	return retryCtx.AttemptCount < rc.maxRetries
}
