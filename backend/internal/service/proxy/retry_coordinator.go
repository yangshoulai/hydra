package proxy

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/yangshoulai/hydra/internal/endpoint"
)

// RetryCoordinator 重试协调器,控制最大重试次数
type RetryCoordinator struct {
	logger     *slog.Logger
	maxRetries int           // 最大重试次数
	retryDelay time.Duration // 重试延迟
	mu         sync.RWMutex  // 保护配置更新的锁
	debugFn    func() bool
}

// NewRetryCoordinator 创建重试协调器
func NewRetryCoordinator(logger *slog.Logger, maxRetries int, retryDelay time.Duration, debugFn func() bool) *RetryCoordinator {
	if debugFn == nil {
		debugFn = func() bool { return false }
	}
	return &RetryCoordinator{
		logger:     logger,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
		debugFn:    debugFn,
	}
}

func (rc *RetryCoordinator) SetDebugFn(debugFn func() bool) {
	if debugFn == nil {
		debugFn = func() bool { return false }
	}
	rc.mu.Lock()
	rc.debugFn = debugFn
	rc.mu.Unlock()
}

func (rc *RetryCoordinator) isDebugEnabled() bool {
	rc.mu.RLock()
	debugFn := rc.debugFn
	rc.mu.RUnlock()
	if debugFn == nil {
		return false
	}
	return debugFn()
}

type ProxyRouteSnapshot struct {
	ChannelID     uint
	ChannelName   string
	KeyID         uint
	KeyMasked     string
	ModelConfigID uint
	ChannelModel  string
	Model         string
	EndpointType  string
}

// AttemptRecord 单次上游尝试的全量采集信息
//
// 仅在调试模式开启时，body/headers 字段才会被填充（见 proxy_service 的采集逻辑）。
// 用于稍后持久化到 request_log_attempts 表。
type AttemptRecord struct {
	AttemptNum    int
	ChannelID     uint
	ChannelName   string
	ChannelModel  string
	KeyID         uint
	KeyName       string // 密钥备注（ChannelKey.Remark），便于展示
	KeyMasked     string
	ModelConfigID uint
	UpstreamURL   string

	StartedAt  time.Time
	DurationMS int64

	UpstreamStatusCode      int
	UpstreamRequestHeaders  http.Header
	UpstreamRequestBody     []byte
	UpstreamResponseHeaders http.Header
	UpstreamResponseBody    []byte

	Success      bool
	FailureType  FailureType
	FailureScope FailureScope
	FailureStage string
	ErrorMessage string
}

// ProxyContext 代理上下文（包含请求元信息与重试状态）
type ProxyContext struct {
	TraceID         string            // 追踪 ID
	Model           string            // 统一模型名
	IsStreamRequest bool              // 是否流式请求
	Endpoint        endpoint.Endpoint // 当前端点
	RequestBody     []byte            // 原始请求体

	// 客户端侧日志采集（仅调试模式使用 headers / response body）
	RequestHeaders http.Header // 入口处客户端请求头快照（脱敏前原文）
	ResponseBody   []byte      // 最终转发给客户端的响应体

	// Token 统计（成功后回填，用于请求日志持久化）
	PromptTokens     int64
	CompletionTokens int64

	AttemptCount     int              // 尝试次数
	RouteAttempts    int              // 实际路由次数（含最终成功或最终失败的尝试）
	Attempts         []*AttemptRecord // 按顺序追加的每次尝试详情
	FailedChannelIDs []uint           // 已失败的 Channel ID
	FailedModelIDs   []uint           // 已失败的模型配置 ID
	FailedKeyIDs     []uint           // 已失败的 Key ID
	LastError        error            // 最后一次错误
	LastFailureType  FailureType      // 最后一次故障类型
	LastFailureScope FailureScope
	LastFailureStage string
	LastRoute        *ProxyRouteSnapshot
	StartTime        time.Time // 整体请求开始时间
	AttemptStartTime time.Time // 当前尝试开始时间
}

// NewProxyContext 创建代理上下文
func NewProxyContext(traceID, model string, isStreamRequest bool, ep endpoint.Endpoint, requestBody []byte) *ProxyContext {
	return &ProxyContext{
		TraceID:          traceID,
		Model:            model,
		IsStreamRequest:  isStreamRequest,
		Endpoint:         ep,
		RequestBody:      requestBody,
		AttemptCount:     0,
		RouteAttempts:    0,
		Attempts:         make([]*AttemptRecord, 0),
		FailedChannelIDs: make([]uint, 0),
		FailedModelIDs:   make([]uint, 0),
		FailedKeyIDs:     make([]uint, 0),
		LastFailureType:  FailureTypeNone,
		LastFailureScope: FailureScopeNone,
		LastFailureStage: "",
		StartTime:        time.Now(),
		AttemptStartTime: time.Now(),
	}
}

// CurrentAttempt 返回最近一次 append 的 AttemptRecord；不存在时返回 nil
func (p *ProxyContext) CurrentAttempt() *AttemptRecord {
	if len(p.Attempts) == 0 {
		return nil
	}
	return p.Attempts[len(p.Attempts)-1]
}

// ShouldRetry 判断是否应该继续重试
// 只要没超过最大重试次数就应该继续尝试下一个渠道
func (rc *RetryCoordinator) ShouldRetry(proxyCtx *ProxyContext) bool {
	rc.mu.RLock()
	maxRetries := rc.maxRetries
	rc.mu.RUnlock()

	// 只检查是否超过最大重试次数
	// 故障类型（hard/soft）只用于熔断器记录，不影响重试决策
	if proxyCtx.AttemptCount >= maxRetries {
		if rc.isDebugEnabled() {
			rc.logger.Info("重试已结束：达到最大重试次数",
				slog.String("trace_id", proxyCtx.TraceID),
				slog.Int("retry_count", proxyCtx.AttemptCount),
				slog.Int("max_retry", maxRetries),
			)
		}
		return false
	}

	return true
}

// RecordAttempt 记录一次尝试
func (rc *RetryCoordinator) RecordAttempt(proxyCtx *ProxyContext, channelID uint, channelName string, modelConfigID uint, keyID uint, err error, failureType FailureType) {
	proxyCtx.AttemptCount++
	proxyCtx.LastError = err
	proxyCtx.LastFailureType = failureType
	proxyCtx.LastFailureStage = ""

	// 记录失败的 Channel
	if !rc.containsChannel(proxyCtx.FailedChannelIDs, channelID) {
		proxyCtx.FailedChannelIDs = append(proxyCtx.FailedChannelIDs, channelID)
	}
	if modelConfigID != 0 && !rc.containsUint(proxyCtx.FailedModelIDs, modelConfigID) {
		proxyCtx.FailedModelIDs = append(proxyCtx.FailedModelIDs, modelConfigID)
	}
	if keyID != 0 && !rc.containsUint(proxyCtx.FailedKeyIDs, keyID) {
		proxyCtx.FailedKeyIDs = append(proxyCtx.FailedKeyIDs, keyID)
	}

	if rc.isDebugEnabled() {
		rc.logger.Info("记录重试",
			slog.String("trace_id", proxyCtx.TraceID),
			slog.Int("retry_count", proxyCtx.AttemptCount),
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("channel_name", channelName),
			slog.Uint64("model_config_id", uint64(modelConfigID)),
			slog.Uint64("key_id", uint64(keyID)),
			slog.String("failure_type", string(failureType)),
			slog.Int("failed_channels_count", len(proxyCtx.FailedChannelIDs)),
			slog.Int("failed_models_count", len(proxyCtx.FailedModelIDs)),
			slog.Int("failed_keys_count", len(proxyCtx.FailedKeyIDs)),
		)
	}
}

// WaitBeforeRetry 在重试前等待
func (rc *RetryCoordinator) WaitBeforeRetry(ctx context.Context, proxyCtx *ProxyContext) error {
	if rc.retryDelay <= 0 {
		return nil
	}

	// 使用指数退避策略
	delay := rc.calculateDelay(proxyCtx.AttemptCount)

	if rc.isDebugEnabled() {
		rc.logger.Debug("等待重试",
			slog.String("trace_id", proxyCtx.TraceID),
			slog.Int("retry_count", proxyCtx.AttemptCount),
			slog.Duration("delay", delay),
		)
	}

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
