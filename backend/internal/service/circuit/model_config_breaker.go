package circuit

import (
	"sync"
	"time"
)

// ModelConfigState 表示模型配置的熔断状态
type ModelConfigState string

const (
	ModelConfigStateActive   ModelConfigState = "active"   // 正常状态
	ModelConfigStateCooling  ModelConfigState = "cooling"  // 冷却中
	ModelConfigStateDisabled ModelConfigState = "disabled" // 禁用/不存在
)

// ModelConfigBreaker 模型配置级别熔断器
type ModelConfigBreaker struct {
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

// NewModelConfigBreaker 创建模型配置熔断器
func NewModelConfigBreaker(configID uint, channelID uint, failureThreshold int, coolingDuration time.Duration) *ModelConfigBreaker {
	return &ModelConfigBreaker{
		configID:         configID,
		channelID:        channelID,
		state:            ModelConfigStateActive,
		failureCount:     0,
		lastFailure:      time.Time{},
		lastSuccess:      time.Time{},
		failureThreshold: failureThreshold,
		coolingDuration:  coolingDuration,
	}
}

// RecordSuccess 记录成功请求
func (mb *ModelConfigBreaker) RecordSuccess() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.lastSuccess = time.Now()
	mb.failureCount = 0

	if mb.state == ModelConfigStateCooling {
		mb.state = ModelConfigStateActive
	}
}

// RecordFailure 记录失败请求
func (mb *ModelConfigBreaker) RecordFailure() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.lastFailure = time.Now()
	mb.failureCount++

	if mb.failureCount >= mb.failureThreshold {
		mb.state = ModelConfigStateCooling
	}
}

// IsAvailable 检查模型配置是否可用
func (mb *ModelConfigBreaker) IsAvailable() bool {
	return mb.state == ModelConfigStateActive || (mb.state == ModelConfigStateCooling && time.Since(mb.lastFailure) >= mb.coolingDuration)
}

// UpdateConfig 更新熔断器配置
func (mb *ModelConfigBreaker) UpdateConfig(failureThreshold int, coolingDuration time.Duration) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.failureThreshold = failureThreshold
	mb.coolingDuration = coolingDuration
}
