package circuit

import (
	"sync"
	"time"
)

// KeyState 表示 Key 的熔断状态
type KeyState string

const (
	KeyStateActive   KeyState = "active"    // 正常状态
	KeyStateDead     KeyState = "dead"      // 永久禁用(硬故障)
	KeyStateCooling  KeyState = "cooling"   // 冷却中(软故障)
	KeyStateHalfOpen KeyState = "half_open" // 半开状态(探测恢复)
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

// NewKeyBreaker 创建 Key 熔断器
func NewKeyBreaker(keyID uint, failureThreshold int, coolingDuration time.Duration) *KeyBreaker {
	return &KeyBreaker{
		keyID:            keyID,
		state:            KeyStateActive,
		failureCount:     0,
		lastFailure:      time.Time{},
		lastSuccess:      time.Time{},
		failureThreshold: failureThreshold,
		coolingDuration:  coolingDuration,
	}
}

// RecordSuccess 记录成功请求
func (kb *KeyBreaker) RecordSuccess() {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.lastSuccess = time.Now()
	kb.failureCount = 0

	// 如果处于半开状态,恢复为正常
	if kb.state == KeyStateHalfOpen {
		kb.state = KeyStateActive
	}
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
		if kb.state == KeyStateHalfOpen {
			// 半开状态失败,重新进入冷却
			kb.state = KeyStateCooling
		} else if kb.state == KeyStateActive {
			// 正常状态达到阈值,进入冷却
			kb.state = KeyStateCooling
		}
	}
}

// GetState 获取当前状态
func (kb *KeyBreaker) GetState() KeyState {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	// 检查是否可以从冷却状态转换为半开状态
	if kb.state == KeyStateCooling {
		if time.Since(kb.lastFailure) >= kb.coolingDuration {
			kb.mu.RUnlock()
			kb.mu.Lock()
			// 双重检查
			if kb.state == KeyStateCooling && time.Since(kb.lastFailure) >= kb.coolingDuration {
				kb.state = KeyStateHalfOpen
				kb.failureCount = 0
			}
			kb.mu.Unlock()
			kb.mu.RLock()
		}
	}

	return kb.state
}

// IsAvailable 检查 Key 是否可用
func (kb *KeyBreaker) IsAvailable() bool {
	state := kb.GetState()
	return state == KeyStateActive || state == KeyStateHalfOpen
}

// Reset 重置熔断器(管理员手动重置)
func (kb *KeyBreaker) Reset() {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.state = KeyStateActive
	kb.failureCount = 0
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
