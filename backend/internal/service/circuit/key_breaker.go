package circuit

import (
	"sync"
	"time"
)

// KeyState 表示 Key 的熔断状态
type KeyState string

const (
	KeyStateActive  KeyState = "active"  // 正常状态
	KeyStateDead    KeyState = "dead"    // 永久禁用(硬故障)
	KeyStateCooling KeyState = "cooling" // 冷却中(软故障)
)

// KeyBreaker Key 级别熔断器
type KeyBreaker struct {
	mu sync.RWMutex

	keyID        uint
	state        KeyState
	failureCount int
	lastFailure  time.Time
	lastSuccess  time.Time

	// 配置参数
	failureThreshold int           // 失败阈值
	coolingDuration  time.Duration // 冷却时长
}

// RecordSuccess 记录成功请求
func (kb *KeyBreaker) RecordSuccess() {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.lastSuccess = time.Now()
	kb.failureCount = 0

	kb.state = KeyStateActive
}

// RecordHardFailure 记录硬故障(401/402/403/429 quota exceeded)
func (kb *KeyBreaker) RecordHardFailure() {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.lastFailure = time.Now()
	kb.state = KeyStateDead
}

// RecordSoftFailure 记录软故障(5xx/timeout)
func (kb *KeyBreaker) RecordSoftFailure() {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.lastFailure = time.Now()
	kb.failureCount++

	// 如果连续失败次数达到阈值,进入冷却状态
	if kb.failureCount >= kb.failureThreshold {
		kb.state = KeyStateCooling
	}
}

// IsAvailable 检查 Key 是否可用
func (kb *KeyBreaker) IsAvailable() bool {
	state := kb.state
	return state == KeyStateActive || (state == KeyStateCooling && time.Since(kb.lastFailure) >= kb.coolingDuration)
}

// GetStats 获取统计信息
func (kb *KeyBreaker) GetStats() map[string]interface{} {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	return map[string]interface{}{
		"key_id":        kb.keyID,
		"state":         string(kb.state),
		"failure_count": kb.failureCount,
		"last_failure":  kb.lastFailure,
		"last_success":  kb.lastSuccess,
	}
}

// UpdateConfig 更新熔断器配置
func (kb *KeyBreaker) UpdateConfig(failureThreshold int, coolingDuration time.Duration) {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.failureThreshold = failureThreshold
	kb.coolingDuration = coolingDuration
}
