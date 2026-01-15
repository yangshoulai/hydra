package circuit

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/config"
	"gorm.io/gorm"
)

// Manager 熔断器管理器
type Manager struct {
	mu sync.RWMutex

	db               *gorm.DB
	logger           *slog.Logger
	keyRepo          *repository.KeyRepository
	channelRepo      *repository.ChannelRepository
	settingService   *config.SettingService // 配置服务，用于获取最新配置
	failureThreshold int
	coolingDuration  time.Duration

	keyBreakers     map[uint]*KeyBreaker
	channelBreakers map[uint]*ChannelBreaker

	probeInterval time.Duration // 探测间隔
	stopChan      chan struct{}
}

// NewManager 创建熔断器管理器
func NewManager(
	db *gorm.DB,
	logger *slog.Logger,
	keyRepo *repository.KeyRepository,
	channelRepo *repository.ChannelRepository,
	settingService *config.SettingService,
	failureThreshold int,
	coolingDuration time.Duration,
	probeInterval time.Duration,
) *Manager {
	return &Manager{
		db:               db,
		logger:           logger,
		keyRepo:          keyRepo,
		channelRepo:      channelRepo,
		settingService:   settingService,
		failureThreshold: failureThreshold,
		coolingDuration:  coolingDuration,
		probeInterval:    probeInterval,
		keyBreakers:      make(map[uint]*KeyBreaker),
		channelBreakers:  make(map[uint]*ChannelBreaker),
		stopChan:         make(chan struct{}),
	}
}

// GetKeyBreaker 获取或创建 Key 熔断器
func (m *Manager) GetKeyBreaker(keyID uint) *KeyBreaker {
	m.mu.RLock()
	breaker, exists := m.keyBreakers[keyID]
	m.mu.RUnlock()

	if exists {
		return breaker
	}

	// 创建新的熔断器
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if breaker, exists := m.keyBreakers[keyID]; exists {
		return breaker
	}

	breaker = NewKeyBreaker(keyID, m.failureThreshold, m.coolingDuration)
	m.keyBreakers[keyID] = breaker

	m.logger.Info("created key breaker",
		slog.Uint64("key_id", uint64(keyID)),
	)

	return breaker
}

// GetChannelBreaker 获取或创建 Channel 熔断器
func (m *Manager) GetChannelBreaker(channelID uint) *ChannelBreaker {
	m.mu.RLock()
	breaker, exists := m.channelBreakers[channelID]
	m.mu.RUnlock()

	if exists {
		return breaker
	}

	// 创建新的熔断器
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if breaker, exists := m.channelBreakers[channelID]; exists {
		return breaker
	}

	breaker = NewChannelBreaker(channelID, m.failureThreshold, m.coolingDuration)
	m.channelBreakers[channelID] = breaker

	m.logger.Info("created channel breaker",
		slog.Uint64("channel_id", uint64(channelID)),
	)

	return breaker
}

// RecordKeySuccess 记录 Key 成功
func (m *Manager) RecordKeySuccess(keyID uint, channelID uint) {
	keyBreaker := m.GetKeyBreaker(keyID)
	keyBreaker.RecordSuccess()

	channelBreaker := m.GetChannelBreaker(channelID)
	channelBreaker.RecordSuccess()

	// 异步更新数据库
	go m.updateKeyStatus(keyID, "active")
}

// RecordKeyHardFailure 记录 Key 硬故障
func (m *Manager) RecordKeyHardFailure(keyID uint, channelID uint) {
	keyBreaker := m.GetKeyBreaker(keyID)
	keyBreaker.RecordHardFailure()

	m.logger.Warn("key hard failure",
		slog.Uint64("key_id", uint64(keyID)),
		slog.Uint64("channel_id", uint64(channelID)),
	)

	// 异步更新数据库
	go m.updateKeyStatus(keyID, "dead")
}

// RecordKeySoftFailure 记录 Key 软故障
func (m *Manager) RecordKeySoftFailure(keyID uint, channelID uint) {
	keyBreaker := m.GetKeyBreaker(keyID)
	keyBreaker.RecordSoftFailure()

	channelBreaker := m.GetChannelBreaker(channelID)
	channelBreaker.RecordFailure()

	// 如果进入冷却状态,更新数据库
	if keyBreaker.GetState() == KeyStateCooling {
		m.logger.Warn("key entering cooling state",
			slog.Uint64("key_id", uint64(keyID)),
			slog.Uint64("channel_id", uint64(channelID)),
		)
		go m.updateKeyStatus(keyID, "cooling")
	}
}

// IsKeyAvailable 检查 Key 是否可用
func (m *Manager) IsKeyAvailable(keyID uint) bool {
	breaker := m.GetKeyBreaker(keyID)
	return breaker.IsAvailable()
}

// IsChannelAvailable 检查 Channel 是否可用
func (m *Manager) IsChannelAvailable(channelID uint) bool {
	breaker := m.GetChannelBreaker(channelID)
	return breaker.IsAvailable()
}

// ResetKey 重置 Key 熔断器
func (m *Manager) ResetKey(keyID uint) {
	breaker := m.GetKeyBreaker(keyID)
	breaker.Reset()

	m.logger.Info("key breaker reset",
		slog.Uint64("key_id", uint64(keyID)),
	)

	// 更新数据库
	go m.updateKeyStatus(keyID, "active")
}

// StartProbeScheduler 启动探测调度器
func (m *Manager) StartProbeScheduler() {
	ticker := time.NewTicker(m.probeInterval)
	defer ticker.Stop()

	m.logger.Info("circuit breaker probe scheduler started",
		slog.Duration("interval", m.probeInterval),
	)

	for {
		select {
		case <-ticker.C:
			m.probeHalfOpenKeys()
		case <-m.stopChan:
			m.logger.Info("circuit breaker probe scheduler stopped")
			return
		}
	}
}

// Stop 停止管理器
func (m *Manager) Stop() {
	close(m.stopChan)
}

// probeHalfOpenKeys 探测半开状态的 Key
func (m *Manager) probeHalfOpenKeys() {
	m.mu.RLock()
	halfOpenKeys := make([]uint, 0)
	for keyID, breaker := range m.keyBreakers {
		if breaker.GetState() == KeyStateHalfOpen {
			halfOpenKeys = append(halfOpenKeys, keyID)
		}
	}
	m.mu.RUnlock()

	if len(halfOpenKeys) == 0 {
		return
	}

	m.logger.Info("probing half-open keys",
		slog.Int("count", len(halfOpenKeys)),
	)

	for _, keyID := range halfOpenKeys {
		go m.probeKey(keyID)
	}
}

// probeKey 探测单个 Key
func (m *Manager) probeKey(keyID uint) {
	// TODO: 实现实际的探测逻辑(调用上游 API)
	// 这里简化处理,假设探测成功
	m.logger.Debug("probing key",
		slog.Uint64("key_id", uint64(keyID)),
	)

	// 探测成功后,状态会自动转换为 Active
	// 如果探测失败,状态会重新转换为 Cooling
}

// updateKeyStatus 更新数据库中的 Key 状态
func (m *Manager) updateKeyStatus(keyID uint, status string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := m.keyRepo.UpdateStatus(ctx, keyID, status); err != nil {
		m.logger.Error("failed to update key status in database",
			slog.Uint64("key_id", uint64(keyID)),
			slog.String("status", status),
			slog.String("error", err.Error()),
		)
	}
}

// GetAllStats 获取所有熔断器统计信息
func (m *Manager) GetAllStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keyStats := make([]map[string]interface{}, 0, len(m.keyBreakers))
	for _, breaker := range m.keyBreakers {
		keyStats = append(keyStats, breaker.GetStats())
	}

	channelStats := make([]map[string]interface{}, 0, len(m.channelBreakers))
	for _, breaker := range m.channelBreakers {
		channelStats = append(channelStats, breaker.GetStats())
	}

	return map[string]interface{}{
		"keys":     keyStats,
		"channels": channelStats,
	}
}

// OnConfigChanged 配置变更回调
func (m *Manager) OnConfigChanged(ctx context.Context, category string) {
	if category != "circuit_breaker" {
		return
	}

	// 从配置服务获取最新的熔断器配置
	failureThreshold := m.settingService.GetInt(ctx, "circuit_breaker_failure_threshold", m.failureThreshold)
	coolingDuration := m.settingService.GetDuration(ctx, "circuit_breaker_cooling_duration", m.coolingDuration)

	m.mu.Lock()

	// 更新配置
	oldFailureThreshold := m.failureThreshold
	oldCoolingDuration := m.coolingDuration
	m.failureThreshold = failureThreshold
	m.coolingDuration = coolingDuration

	// 更新所有现有的熔断器配置
	for _, breaker := range m.keyBreakers {
		breaker.UpdateConfig(failureThreshold, coolingDuration)
	}
	for _, breaker := range m.channelBreakers {
		breaker.UpdateConfig(failureThreshold, coolingDuration)
	}

	m.mu.Unlock()

	m.logger.Info("circuit breaker config updated",
		slog.Int("old_failure_threshold", oldFailureThreshold),
		slog.Int("new_failure_threshold", failureThreshold),
		slog.Duration("old_cooling_duration", oldCoolingDuration),
		slog.Duration("new_cooling_duration", coolingDuration),
	)
}
