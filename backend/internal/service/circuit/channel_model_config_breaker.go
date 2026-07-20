package circuit

import (
	"sync"
	"time"
)

// ModelConfigState 表示模型配置的熔断状态
type ModelConfigState string

const (
	ModelConfigStateActive   ModelConfigState = "active"  // 正常状态
	ModelConfigStateCooling  ModelConfigState = "cooling" // 冷却中（仅内存）
	ModelConfigStateHalfOpen ModelConfigState = "half_open"
	ModelConfigStateInactive ModelConfigState = "inactive" // 已停用
)

// ChannelModelConfigBreaker 渠道模型配置级别熔断器
type ChannelModelConfigBreaker struct {
	mu sync.RWMutex

	configID  uint
	channelID uint
	state     ModelConfigState

	failureCount  int
	lastFailure   time.Time
	lastSuccess   time.Time
	probeInFlight bool

	failureThreshold int
	coolingDuration  time.Duration
}

// RecordSuccess 记录成功请求
func (mb *ChannelModelConfigBreaker) RecordSuccess() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.lastSuccess = time.Now()
	mb.failureCount = 0
	mb.probeInFlight = false

	mb.state = ModelConfigStateActive
}

// RecordFailure 记录失败请求，返回状态变更快照。
func (mb *ChannelModelConfigBreaker) RecordFailure() (oldState ModelConfigState, newState ModelConfigState, failureCount int, lastFailure time.Time) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	oldState = mb.state
	mb.lastFailure = time.Now()
	mb.failureCount++
	mb.probeInFlight = false

	if oldState == ModelConfigStateHalfOpen || mb.failureCount >= mb.failureThreshold {
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
		return !mb.probeInFlight && time.Since(mb.lastFailure) >= (mb.coolingDuration+time.Duration(additionalSeconds)*time.Second)
	}
	return false
}

// TryAcquireProbe 在冷却到期时领取唯一的 half-open 探测名额。
// 返回值: (是否可用, 是否领取了 half-open 名额)。
func (mb *ChannelModelConfigBreaker) TryAcquireProbe() (bool, bool) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if mb.state == ModelConfigStateActive {
		return true, false
	}
	if mb.state != ModelConfigStateCooling {
		return false, false
	}

	additionalSeconds := min(max(mb.failureCount-mb.failureThreshold, 0), 5) * 60
	if time.Since(mb.lastFailure) < (mb.coolingDuration + time.Duration(additionalSeconds)*time.Second) {
		return false, false
	}
	if mb.probeInFlight {
		return false, false
	}

	mb.state = ModelConfigStateHalfOpen
	mb.probeInFlight = true
	return true, true
}

// ReleaseProbe 释放未真正发出的 half-open 探测名额。
func (mb *ChannelModelConfigBreaker) ReleaseProbe() {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	if mb.state == ModelConfigStateHalfOpen && mb.probeInFlight {
		mb.state = ModelConfigStateCooling
		mb.probeInFlight = false
	}
}

// UpdateConfig 更新熔断器配置
func (mb *ChannelModelConfigBreaker) UpdateConfig(failureThreshold int, coolingDuration time.Duration) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.failureThreshold = failureThreshold
	mb.coolingDuration = coolingDuration
}
