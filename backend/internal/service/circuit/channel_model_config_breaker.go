package circuit

import (
	"sync"
	"time"
)

// ModelConfigState 表示模型配置的熔断状态
type ModelConfigState string

const (
	ModelConfigStateActive   ModelConfigState = "active"   // 正常状态
	ModelConfigStateCooling  ModelConfigState = "cooling"  // 冷却中（仅内存）
	ModelConfigStateInactive ModelConfigState = "inactive" // 已停用
)

// ChannelModelConfigBreaker 渠道模型配置级别熔断器
type ChannelModelConfigBreaker struct {
	mu sync.RWMutex

	configID  uint
	channelID uint
	state     ModelConfigState

	failureCount int
	lastFailure  time.Time
	lastSuccess  time.Time

	failureThreshold int
	coolingDuration  time.Duration
}

// RecordSuccess 记录成功请求
func (mb *ChannelModelConfigBreaker) RecordSuccess() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.lastSuccess = time.Now()
	mb.failureCount = 0

	if mb.state == ModelConfigStateCooling {
		mb.state = ModelConfigStateActive
	}
}

// RecordFailure 记录失败请求，返回状态变更快照。
func (mb *ChannelModelConfigBreaker) RecordFailure() (oldState ModelConfigState, newState ModelConfigState, failureCount int, lastFailure time.Time) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	oldState = mb.state
	mb.lastFailure = time.Now()
	mb.failureCount++

	if mb.failureCount >= mb.failureThreshold {
		mb.state = ModelConfigStateCooling
	}
	return oldState, mb.state, mb.failureCount, mb.lastFailure
}

// IsAvailable 检查模型配置是否可用
func (mb *ChannelModelConfigBreaker) IsAvailable() bool {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	if mb.state == ModelConfigStateActive {
		return true
	}
	if mb.state == ModelConfigStateCooling {
		// 超过阈值则延长冷却时间，每多一次失败，则延长一分钟，最多可以额外延长 5 分钟
		additionalSeconds := min(max(mb.failureCount-mb.failureThreshold, 0), 5) * 60
		return time.Since(mb.lastFailure) >= (mb.coolingDuration + time.Duration(additionalSeconds)*time.Second)
	}
	return false
}

// UpdateConfig 更新熔断器配置
func (mb *ChannelModelConfigBreaker) UpdateConfig(failureThreshold int, coolingDuration time.Duration) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.failureThreshold = failureThreshold
	mb.coolingDuration = coolingDuration
}
