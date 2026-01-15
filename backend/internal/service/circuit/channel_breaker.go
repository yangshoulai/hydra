package circuit

import (
	"sync"
	"time"
)

// ChannelState 表示 Channel 的熔断状态
type ChannelState string

const (
	ChannelStateActive   ChannelState = "active"   // 正常状态
	ChannelStateCooling  ChannelState = "cooling"  // 冷却中
	ChannelStateDisabled ChannelState = "disabled" // 禁用(管理员操作)
)

// ChannelBreaker Channel 级别熔断器
type ChannelBreaker struct {
	mu sync.RWMutex

	channelID    uint
	state        ChannelState
	failureCount int
	lastFailure  time.Time
	lastSuccess  time.Time

	// 配置参数
	failureThreshold int           // 失败阈值
	coolingDuration  time.Duration // 冷却时长
}

// NewChannelBreaker 创建 Channel 熔断器
func NewChannelBreaker(channelID uint, failureThreshold int, coolingDuration time.Duration) *ChannelBreaker {
	return &ChannelBreaker{
		channelID:        channelID,
		state:            ChannelStateActive,
		failureCount:     0,
		lastFailure:      time.Time{},
		lastSuccess:      time.Time{},
		failureThreshold: failureThreshold,
		coolingDuration:  coolingDuration,
	}
}

// RecordSuccess 记录成功请求
func (cb *ChannelBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastSuccess = time.Now()
	cb.failureCount = 0

	// 如果处于冷却状态,恢复为正常
	if cb.state == ChannelStateCooling {
		cb.state = ChannelStateActive
	}
}

// RecordFailure 记录失败请求
func (cb *ChannelBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailure = time.Now()
	cb.failureCount++

	// 如果连续失败次数达到阈值,进入冷却状态
	if cb.failureCount >= cb.failureThreshold && cb.state == ChannelStateActive {
		cb.state = ChannelStateCooling
	}
}

// GetState 获取当前状态
func (cb *ChannelBreaker) GetState() ChannelState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	// 检查是否可以从冷却状态转换为正常状态
	if cb.state == ChannelStateCooling {
		if time.Since(cb.lastFailure) >= cb.coolingDuration {
			cb.mu.RUnlock()
			cb.mu.Lock()
			// 双重检查
			if cb.state == ChannelStateCooling && time.Since(cb.lastFailure) >= cb.coolingDuration {
				cb.state = ChannelStateActive
				cb.failureCount = 0
			}
			cb.mu.Unlock()
			cb.mu.RLock()
		}
	}

	return cb.state
}

// IsAvailable 检查 Channel 是否可用
func (cb *ChannelBreaker) IsAvailable() bool {
	state := cb.GetState()
	return state == ChannelStateActive
}

// Disable 禁用 Channel(管理员操作)
func (cb *ChannelBreaker) Disable() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = ChannelStateDisabled
}

// Enable 启用 Channel(管理员操作)
func (cb *ChannelBreaker) Enable() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == ChannelStateDisabled {
		cb.state = ChannelStateActive
		cb.failureCount = 0
	}
}

// Reset 重置熔断器
func (cb *ChannelBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = ChannelStateActive
	cb.failureCount = 0
}

// GetStats 获取统计信息
func (cb *ChannelBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"channel_id":    cb.channelID,
		"state":         string(cb.state),
		"failure_count": cb.failureCount,
		"last_failure":  cb.lastFailure,
		"last_success":  cb.lastSuccess,
	}
}

// UpdateConfig 更新熔断器配置
func (cb *ChannelBreaker) UpdateConfig(failureThreshold int, coolingDuration time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureThreshold = failureThreshold
	cb.coolingDuration = coolingDuration
}
