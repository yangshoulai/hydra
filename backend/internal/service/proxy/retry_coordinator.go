package proxy

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// RetryCoordinator 重试协调器,控制最大重试次数
type RetryCoordinator struct {
	logger     *slog.Logger
	maxRetries int           // 最大重试次数
	retryDelay time.Duration // 重试延迟
	mu         sync.RWMutex  // 保护配置更新的锁
}

// NewRetryCoordinator 创建重试协调器
func NewRetryCoordinator(logger *slog.Logger, maxRetries int, retryDelay time.Duration) *RetryCoordinator {
	return &RetryCoordinator{
		logger:     logger,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
}

// RetryContext 重试上下文
type RetryContext struct {
	AttemptCount     int         // 尝试次数
	FailedChannelIDs []uint      // 已失败的 Channel ID
	FailedModelIDs   []uint      // 已失败的模型配置 ID
	FailedKeyIDs     []uint      // 已失败的 Key ID
	LastError        error       // 最后一次错误
	LastFailureType  FailureType // 最后一次故障类型
	StartTime        time.Time   // 开始时间
}

// NewRetryContext 创建重试上下文
func NewRetryContext() *RetryContext {
	return &RetryContext{
		AttemptCount:     0,
		FailedChannelIDs: make([]uint, 0),
		FailedModelIDs:   make([]uint, 0),
		FailedKeyIDs:     make([]uint, 0),
		StartTime:        time.Now(),
	}
}

// ShouldRetry 判断是否应该继续重试
// 只要没超过最大重试次数就应该继续尝试下一个渠道
func (rc *RetryCoordinator) ShouldRetry(retryCtx *RetryContext) bool {
	if retryCtx == nil {
		return false
	}

	rc.mu.RLock()
	maxRetries := rc.maxRetries
	rc.mu.RUnlock()

	// 只检查是否超过最大重试次数
	// 故障类型（hard/soft）只用于熔断器记录，不影响重试决策
	if retryCtx.AttemptCount >= maxRetries {
		rc.logger.Warn("达到最大重试次数",
			slog.Int("attempt_count", retryCtx.AttemptCount),
			slog.Int("max_retries", maxRetries),
		)
		return false
	}

	return true
}

// RecordAttempt 记录一次尝试
func (rc *RetryCoordinator) RecordAttempt(retryCtx *RetryContext, channelID uint, channelName string, modelConfigID uint, keyID uint, err error, failureType FailureType, traceID string) {
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
	if modelConfigID != 0 && !rc.containsUint(retryCtx.FailedModelIDs, modelConfigID) {
		retryCtx.FailedModelIDs = append(retryCtx.FailedModelIDs, modelConfigID)
	}
	if keyID != 0 && !rc.containsUint(retryCtx.FailedKeyIDs, keyID) {
		retryCtx.FailedKeyIDs = append(retryCtx.FailedKeyIDs, keyID)
	}

	rc.logger.Info("记录重试",
		slog.String("trace_id", traceID),
		slog.Int("attempt_count", retryCtx.AttemptCount),
		slog.Uint64("channel_id", uint64(channelID)),
		slog.String("channel_name", channelName),
		slog.Uint64("model_config_id", uint64(modelConfigID)),
		slog.Uint64("key_id", uint64(keyID)),
		slog.String("failure_type", string(failureType)),
		slog.Int("failed_channels_count", len(retryCtx.FailedChannelIDs)),
		slog.Int("failed_models_count", len(retryCtx.FailedModelIDs)),
		slog.Int("failed_keys_count", len(retryCtx.FailedKeyIDs)),
	)
}

// WaitBeforeRetry 在重试前等待
func (rc *RetryCoordinator) WaitBeforeRetry(ctx context.Context, retryCtx *RetryContext) error {
	if rc.retryDelay <= 0 {
		return nil
	}

	// 使用指数退避策略
	delay := rc.calculateDelay(retryCtx.AttemptCount)

	rc.logger.Debug("等待重试中",
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

func (rc *RetryCoordinator) containsUint(ids []uint, target uint) bool {
	for _, id := range ids {
		if id == target {
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
	rc.mu.RLock()
	maxRetries := rc.maxRetries
	rc.mu.RUnlock()
	return retryCtx.AttemptCount < maxRetries
}

// UpdateConfig 更新重试配置
func (rc *RetryCoordinator) UpdateConfig(maxRetries int, retryDelay time.Duration) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	oldMaxRetries := rc.maxRetries
	oldRetryDelay := rc.retryDelay

	rc.maxRetries = maxRetries
	rc.retryDelay = retryDelay

	rc.logger.Info("重试协调器配置已更新",
		slog.Int("old_max_retries", oldMaxRetries),
		slog.Int("new_max_retries", maxRetries),
		slog.Duration("old_retry_delay", oldRetryDelay),
		slog.Duration("new_retry_delay", retryDelay),
	)
}
