package circuit

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
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
	modelConfigRepo  *repository.ChannelModelConfigRepository
	settingService   *config.SettingService // 配置服务，用于获取最新配置
	failureThreshold int
	coolingDuration  time.Duration

	keyBreakers         map[uint]*KeyBreaker
	modelConfigBreakers map[uint]*ModelConfigBreaker

	probeInterval      time.Duration // 探测间隔
	probeMaxConcurrent int           // 最大并发探测数
	probing            int32         // 探测进行中标志 (0=空闲, 1=探测中)
	stopChan           chan struct{}
	restartChan        chan struct{} // 重启探测调度器信号
}

// NewManager 创建熔断器管理器
func NewManager(
	db *gorm.DB,
	logger *slog.Logger,
	keyRepo *repository.KeyRepository,
	channelRepo *repository.ChannelRepository,
	modelConfigRepo *repository.ChannelModelConfigRepository,
	settingService *config.SettingService,
	failureThreshold int,
	coolingDuration time.Duration,
	probeInterval time.Duration,
) *Manager {
	return &Manager{
		db:                  db,
		logger:              logger,
		keyRepo:             keyRepo,
		channelRepo:         channelRepo,
		modelConfigRepo:     modelConfigRepo,
		settingService:      settingService,
		failureThreshold:    failureThreshold,
		coolingDuration:     coolingDuration,
		probeInterval:       probeInterval,
		probeMaxConcurrent:  10, // 默认值
		keyBreakers:         make(map[uint]*KeyBreaker),
		modelConfigBreakers: make(map[uint]*ModelConfigBreaker),
		stopChan:            make(chan struct{}),
		restartChan:         make(chan struct{}),
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

	state := KeyStateActive
	failureCount := 0
	lastFailure := time.Time{}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key, err := m.keyRepo.FindByID(ctx, keyID)
	if err != nil {
		m.logger.Error("从数据库获取密钥状态失败",
			slog.Uint64("key_id", uint64(keyID)),
			slog.String("error", err.Error()),
		)
	} else if key != nil {
		switch key.Status {
		case "cooling":
			state = KeyStateCooling
			failureCount = m.failureThreshold
			if key.CoolingAt != nil {
				lastFailure = key.CoolingAt.Add(-m.coolingDuration)
			} else {
				lastFailure = time.Now()
			}
		case "dead", "disabled":
			state = KeyStateDead
			failureCount = m.failureThreshold
		}
	}

	// 创建新的熔断器（带数据库状态）
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if breaker, exists := m.keyBreakers[keyID]; exists {
		return breaker
	}

	breaker = &KeyBreaker{
		keyID:            keyID,
		state:            state,
		failureCount:     failureCount,
		lastFailure:      lastFailure,
		failureThreshold: m.failureThreshold,
		coolingDuration:  m.coolingDuration,
	}
	m.keyBreakers[keyID] = breaker

	m.logger.Debug("创建密钥熔断器",
		slog.Uint64("key_id", uint64(keyID)),
		slog.String("state", string(state)),
	)

	return breaker
}

// GetModelConfigBreaker 获取或创建模型配置熔断器
func (m *Manager) GetModelConfigBreaker(modelConfigID uint, channelID uint) *ModelConfigBreaker {
	m.mu.RLock()
	breaker, exists := m.modelConfigBreakers[modelConfigID]
	m.mu.RUnlock()

	if exists {
		return breaker
	}

	state := ModelConfigStateActive
	failureCount := 0
	lastFailure := time.Time{}
	actualChannelID := channelID

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	config, err := m.modelConfigRepo.FindByID(ctx, modelConfigID)
	if err != nil {
		m.logger.Error("从数据库获取模型配置状态失败",
			slog.Uint64("model_config_id", uint64(modelConfigID)),
			slog.String("error", err.Error()),
		)
	} else if config != nil {
		actualChannelID = config.ChannelID
		switch config.Status {
		case "cooling":
			state = ModelConfigStateCooling
			failureCount = m.failureThreshold
			if config.CoolingAt != nil {
				lastFailure = *config.CoolingAt
			} else {
				lastFailure = time.Now()
			}
		case "disabled", "non_exist":
			state = ModelConfigStateDisabled
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if breaker, exists := m.modelConfigBreakers[modelConfigID]; exists {
		return breaker
	}

	breaker = &ModelConfigBreaker{
		configID:         modelConfigID,
		channelID:        actualChannelID,
		state:            state,
		failureCount:     failureCount,
		lastFailure:      lastFailure,
		failureThreshold: m.failureThreshold,
		coolingDuration:  m.coolingDuration,
	}
	m.modelConfigBreakers[modelConfigID] = breaker

	m.logger.Debug("创建模型配置熔断器",
		slog.Uint64("model_config_id", uint64(modelConfigID)),
		slog.Uint64("channel_id", uint64(actualChannelID)),
		slog.String("state", string(state)),
	)

	return breaker
}

// RecordKeySuccess 记录 Key 成功
func (m *Manager) RecordKeySuccess(keyID uint, channelID uint) {
	keyBreaker := m.GetKeyBreaker(keyID)
	oldState := keyBreaker.state
	keyBreaker.RecordSuccess()

	// 异步更新数据库，如果之前在 cooling 状态，需要清除 cooling_at
	if oldState != keyBreaker.state {
		go m.exitKeyCooling(keyID)
	}

}

// RecordKeyHardFailure 记录 Key 硬故障
func (m *Manager) RecordKeyHardFailure(keyID uint, channelID uint, channelName string, errMsg string) {
	keyBreaker := m.GetKeyBreaker(keyID)
	keyBreaker.RecordHardFailure()

	m.logger.Warn("密钥硬故障",
		slog.Uint64("key_id", uint64(keyID)),
		slog.Uint64("channel_id", uint64(channelID)),
		slog.String("channel_name", channelName),
		slog.String("errMsg", errMsg),
	)

	// 异步更新数据库
	go m.updateKeyStatus(keyID, "dead")
}

// RecordKeySoftFailure 记录 Key 软故障
func (m *Manager) RecordKeySoftFailure(keyID uint, channelID uint, channelName string, errMsg string) {
	keyBreaker := m.GetKeyBreaker(keyID)

	oldState := keyBreaker.state

	keyBreaker.RecordSoftFailure()
	m.logger.Warn("密钥连续["+strconv.Itoa(keyBreaker.failureCount)+"]次失败",
		slog.Uint64("key_id", uint64(keyID)),
		slog.Uint64("channel_id", uint64(channelID)),
		slog.String("channel_name", channelName),
	)

	// 如果进入冷却状态,更新数据库
	if oldState != KeyStateCooling && keyBreaker.state == KeyStateCooling {
		m.logger.Warn("密钥进入冷却状态",
			slog.Uint64("key_id", uint64(keyID)),
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("channel_name", channelName),
			slog.String("errMsg", errMsg),
		)
		go m.enterKeyCooling(keyID)
	}
}

// RecordModelConfigSuccess 记录模型配置成功
func (m *Manager) RecordModelConfigSuccess(modelConfigID uint, channelID uint) {
	breaker := m.GetModelConfigBreaker(modelConfigID, channelID)
	oldState := breaker.state
	breaker.RecordSuccess()

	if oldState != ModelConfigStateActive {
		go m.exitModelConfigCooling(modelConfigID)
	}

}

// RecordModelConfigFailure 记录模型配置失败（统一模型到上游模型的映射调用失败）
func (m *Manager) RecordModelConfigFailure(modelConfigID uint, channelID uint, channelName string, unifiedModel string, upstreamModel string, errMsg string) {
	breaker := m.GetModelConfigBreaker(modelConfigID, channelID)
	oldState := breaker.state
	breaker.RecordFailure()

	m.logger.Warn("模型配置连续["+strconv.Itoa(breaker.failureCount)+"]次失败",
		slog.Uint64("model_config_id", uint64(modelConfigID)),
		slog.Uint64("channel_id", uint64(channelID)),
		slog.String("channel_name", channelName),
		slog.String("unified_model", unifiedModel),
		slog.String("upstream_model", upstreamModel),
		slog.String("errMsg", errMsg),
	)

	if oldState != breaker.state && breaker.state == ModelConfigStateCooling {
		m.logger.Warn("模型配置进入冷却状态",
			slog.Uint64("model_config_id", uint64(modelConfigID)),
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("channel_name", channelName),
			slog.String("unified_model", unifiedModel),
			slog.String("upstream_model", upstreamModel),
		)
		go m.enterModelConfigCooling(modelConfigID)
	}
}

// IsKeyAvailable 检查 Key 是否可用
func (m *Manager) IsKeyAvailable(keyID uint) bool {
	breaker := m.GetKeyBreaker(keyID)
	return breaker.IsAvailable()
}

// IsModelConfigAvailable 检查模型配置是否可用
func (m *Manager) IsModelConfigAvailable(modelConfigID uint) bool {
	breaker := m.GetModelConfigBreaker(modelConfigID, 0)
	return breaker.IsAvailable()
}

// Stop 停止管理器
func (m *Manager) Stop() {
	close(m.stopChan)
}

// keyProbeCandidate 探测候选 Key
type keyProbeCandidate struct {
	keyID       uint
	lastFailure time.Time
}

// updateKeyStatus 更新数据库中的 Key 状态
func (m *Manager) updateKeyStatus(keyID uint, status string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := m.keyRepo.UpdateStatus(ctx, keyID, status); err != nil {
		m.logger.Error("更新数据库中的密钥状态失败",
			slog.Uint64("key_id", uint64(keyID)),
			slog.String("status", status),
			slog.String("error", err.Error()),
		)
	}
}

// enterKeyCooling 设置 Key 进入冷却状态（同时更新 cooling_at 字段）
func (m *Manager) enterKeyCooling(keyID uint) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := m.keyRepo.EnterCooling(ctx, keyID, m.coolingDuration); err != nil {
		m.logger.Error("设置密钥冷却状态失败",
			slog.Uint64("key_id", uint64(keyID)),
			slog.Duration("cooling_duration", m.coolingDuration),
			slog.String("error", err.Error()),
		)
	}
}

// exitKeyCooling 退出冷却状态（清除 cooling_at 字段）
func (m *Manager) exitKeyCooling(keyID uint) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := m.keyRepo.ExitCooling(ctx, keyID); err != nil {
		m.logger.Error("退出密钥冷却状态失败",
			slog.Uint64("key_id", uint64(keyID)),
			slog.String("error", err.Error()),
		)
	}
}

// enterModelConfigCooling 设置模型配置进入冷却状态
func (m *Manager) enterModelConfigCooling(modelConfigID uint) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := m.modelConfigRepo.EnterCooling(ctx, modelConfigID); err != nil {
		m.logger.Error("设置模型配置冷却状态失败",
			slog.Uint64("model_config_id", uint64(modelConfigID)),
			slog.String("error", err.Error()),
		)
	}
}

// exitModelConfigCooling 退出模型配置冷却状态
func (m *Manager) exitModelConfigCooling(modelConfigID uint) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := m.modelConfigRepo.ExitCooling(ctx, modelConfigID); err != nil {
		m.logger.Error("退出模型配置冷却状态失败",
			slog.Uint64("model_config_id", uint64(modelConfigID)),
			slog.String("error", err.Error()),
		)
	}
}

// OnConfigChanged 配置变更回调
func (m *Manager) OnConfigChanged(ctx context.Context, category string) {
	if category != "circuit_breaker" {
		return
	}

	// 从配置服务获取最新的熔断器配置
	failureThreshold := m.settingService.GetInt(ctx, models.SettingCircuitBreakerFailureThreshold, m.failureThreshold)
	coolingDuration := m.settingService.GetDuration(ctx, models.SettingCircuitBreakerCoolingDuration, m.coolingDuration)
	probeInterval := m.settingService.GetDuration(ctx, models.SettingCircuitBreakerProbeInterval, m.probeInterval)
	probeMaxConcurrent := m.settingService.GetInt(ctx, models.SettingCircuitBreakerProbeMaxConcurrent, m.probeMaxConcurrent)

	m.mu.Lock()

	// 更新配置
	oldFailureThreshold := m.failureThreshold
	oldCoolingDuration := m.coolingDuration
	oldProbeInterval := m.probeInterval
	oldProbeMaxConcurrent := m.probeMaxConcurrent

	m.failureThreshold = failureThreshold
	m.coolingDuration = coolingDuration
	m.probeInterval = probeInterval
	m.probeMaxConcurrent = probeMaxConcurrent

	// 更新所有现有的熔断器配置
	for _, breaker := range m.keyBreakers {
		breaker.UpdateConfig(failureThreshold, coolingDuration)
	}
	for _, breaker := range m.modelConfigBreakers {
		breaker.UpdateConfig(failureThreshold, coolingDuration)
	}

	m.mu.Unlock()

	m.logger.Info("熔断器配置已更新",
		slog.Int("old_failure_threshold", oldFailureThreshold),
		slog.Int("new_failure_threshold", failureThreshold),
		slog.Duration("old_cooling_duration", oldCoolingDuration),
		slog.Duration("new_cooling_duration", coolingDuration),
		slog.Duration("old_probe_interval", oldProbeInterval),
		slog.Duration("new_probe_interval", probeInterval),
		slog.Int("old_probe_max_concurrent", oldProbeMaxConcurrent),
		slog.Int("new_probe_max_concurrent", probeMaxConcurrent),
	)

	// 如果探测间隔发生变化，重启探测调度器
	if oldProbeInterval != probeInterval {
		m.logger.Info("探测间隔已变更，重启调度器")
		select {
		case m.restartChan <- struct{}{}:
		default:
			m.logger.Warn("发送重启信号失败，通道可能已满")
		}
	}
}

// RemoveKeyBreaker 移除 Key 熔断器
func (m *Manager) RemoveKeyBreaker(keyID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.keyBreakers[keyID]; exists {
		delete(m.keyBreakers, keyID)
		m.logger.Info("移除密钥熔断器",
			slog.Uint64("key_id", uint64(keyID)),
		)
	}
}

// RemoveModelConfigBreaker 移除模型配置熔断器
func (m *Manager) RemoveModelConfigBreaker(modelConfigID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.modelConfigBreakers[modelConfigID]; exists {
		delete(m.modelConfigBreakers, modelConfigID)
		m.logger.Info("移除模型配置熔断器",
			slog.Uint64("model_config_id", uint64(modelConfigID)),
		)
	}
}

// RemoveChannelBreakersAndKeys 移除渠道相关的熔断器（密钥 + 模型配置）
func (m *Manager) RemoveChannelBreakersAndKeys(channelID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查询该渠道下的所有密钥
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	keys, err := m.keyRepo.FindByChannelID(ctx, channelID)
	if err != nil {
		m.logger.Error("查询渠道密钥失败",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("error", err.Error()),
		)
		return
	}

	// 移除该渠道下所有密钥的熔断器
	removedCount := 0
	for _, key := range keys {
		if _, exists := m.keyBreakers[key.ID]; exists {
			delete(m.keyBreakers, key.ID)
			removedCount++
		}
	}

	if removedCount > 0 {
		m.logger.Info("移除渠道下密钥熔断器",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.Int("count", removedCount),
		)
	}

	// 移除该渠道下所有模型配置熔断器
	configs, err := m.modelConfigRepo.FindByChannelID(ctx, channelID)
	if err != nil {
		m.logger.Error("查询渠道模型配置失败",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("error", err.Error()),
		)
		return
	}

	removedModelConfigCount := 0
	for _, cfg := range configs {
		if _, exists := m.modelConfigBreakers[cfg.ID]; exists {
			delete(m.modelConfigBreakers, cfg.ID)
			removedModelConfigCount++
		}
	}

	if removedModelConfigCount > 0 {
		m.logger.Info("移除渠道下模型配置熔断器",
			slog.Uint64("channel_id", uint64(channelID)),
			slog.Int("count", removedModelConfigCount),
		)
	}
}

// CleanupOrphanBreakers 清理孤儿熔断器（数据库中不存在的密钥或模型配置）
func (m *Manager) CleanupOrphanBreakers(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取数据库中所有的密钥、模型配置ID（不限制状态）
	existingKeyIDs := make(map[uint]bool)
	existingModelConfigIDs := make(map[uint]bool)

	// 查询所有密钥（不限制状态）
	keys, err := m.keyRepo.FindAll(ctx)
	if err != nil {
		m.logger.Error("查询密钥失败",
			slog.String("error", err.Error()),
		)
		return
	}
	for _, key := range keys {
		existingKeyIDs[key.ID] = true
	}

	// 查询所有模型配置ID（不限制状态）
	var modelConfigIDs []uint
	if err := m.db.WithContext(ctx).Model(&models.ChannelModelConfig{}).Pluck("id", &modelConfigIDs).Error; err != nil {
		m.logger.Error("查询模型配置失败",
			slog.String("error", err.Error()),
		)
		return
	}
	for _, id := range modelConfigIDs {
		existingModelConfigIDs[id] = true
	}

	// 清理孤儿密钥熔断器（数据库中不存在的密钥）
	var removedKeyBreakers, removedModelConfigBreakers int

	for keyID := range m.keyBreakers {
		if !existingKeyIDs[keyID] {
			delete(m.keyBreakers, keyID)
			removedKeyBreakers++
			m.logger.Debug("清理孤儿密钥熔断器",
				slog.Uint64("key_id", uint64(keyID)),
			)
		}
	}

	// 清理孤儿模型配置熔断器（数据库中不存在的模型配置）
	for cfgID := range m.modelConfigBreakers {
		if !existingModelConfigIDs[cfgID] {
			delete(m.modelConfigBreakers, cfgID)
			removedModelConfigBreakers++
			m.logger.Debug("清理孤儿模型配置熔断器",
				slog.Uint64("model_config_id", uint64(cfgID)),
			)
		}
	}

	if removedKeyBreakers > 0 || removedModelConfigBreakers > 0 {
		m.logger.Info("清理孤儿熔断器完成",
			slog.Int("removed_key_breakers", removedKeyBreakers),
			slog.Int("removed_model_config_breakers", removedModelConfigBreakers),
			slog.Int("remaining_key_breakers", len(m.keyBreakers)),
			slog.Int("remaining_model_config_breakers", len(m.modelConfigBreakers)),
		)
	}
}
