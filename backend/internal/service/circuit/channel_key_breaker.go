package circuit

import (
	"sync"
	"time"
)

// KeyState 表示 Key 的熔断状态
type KeyState string

const (
	KeyStateActive   KeyState = "active"   // 正常状态
	KeyStateInactive KeyState = "inactive" // 已停用（硬故障/人工停用）
	KeyStateCooling  KeyState = "cooling"  // 冷却中（软故障，仅内存）
)

// ChannelKeyBreaker 渠道密钥级别熔断器
type ChannelKeyBreaker struct {
	mu sync.RWMutex

	keyID        uint
	channelID    uint
	state        KeyState
	failureCount int
	lastFailure  time.Time
	lastSuccess  time.Time

	// 配置参数
	failureThreshold int           // 失败阈值
	coolingDuration  time.Duration // 冷却时长
}

// RecordSuccess 记录成功请求
func (kb *ChannelKeyBreaker) RecordSuccess() {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.lastSuccess = time.Now()
	kb.failureCount = 0

	kb.state = KeyStateActive
}

// RecordHardFailure 记录硬故障，返回状态变更快照。
func (kb *ChannelKeyBreaker) RecordHardFailure() (oldState KeyState, newState KeyState, failureCount int, lastFailure time.Time) {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	oldState = kb.state
	kb.lastFailure = time.Now()
	kb.state = KeyStateInactive
	return oldState, kb.state, kb.failureCount, kb.lastFailure
}

// RecordSoftFailure 记录软故障(5xx/timeout)，返回状态变更快照。
func (kb *ChannelKeyBreaker) RecordSoftFailure() (oldState KeyState, newState KeyState, failureCount int, lastFailure time.Time) {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	oldState = kb.state
	kb.lastFailure = time.Now()
	kb.failureCount++

	// 如果连续失败次数达到阈值,进入冷却状态
	if kb.failureCount >= kb.failureThreshold {
		kb.state = KeyStateCooling
	}
	return oldState, kb.state, kb.failureCount, kb.lastFailure
}

// IsAvailable 检查 Key 是否可用
func (kb *ChannelKeyBreaker) IsAvailable() bool {
	state := kb.state
	if state == KeyStateActive {
		return true
	}
	if state == KeyStateCooling {
		// 超过阈值则延长冷却时间，每多一次失败，则延长一分钟，最多可以额外延长 5 分钟
		additionalSeconds := min(max(kb.failureCount-kb.failureThreshold, 0), 5) * 60
		return time.Since(kb.lastFailure) >= (kb.coolingDuration + time.Duration(additionalSeconds)*time.Second)
	}
	return false
}

// UpdateConfig 更新熔断器配置
func (kb *ChannelKeyBreaker) UpdateConfig(failureThreshold int, coolingDuration time.Duration) {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.failureThreshold = failureThreshold
	kb.coolingDuration = coolingDuration
}
