package circuit

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/config"
	"gorm.io/gorm"
)

// CircuitManager 熔断器管理器
type CircuitManager struct {
	mu sync.RWMutex

	db               *gorm.DB
	logger           *slog.Logger
	channelKeyRepo   *repository.ChannelKeyRepository
	modelConfigRepo  *repository.ChannelModelConfigRepository
	settingService   *config.SettingService // 配置服务，用于获取最新配置
	failureThreshold int
	coolingDuration  time.Duration

	keyBreakers         map[uint]*ChannelKeyBreaker
	modelConfigBreakers map[uint]*ChannelModelConfigBreaker
}

// BreakerSnapshot 熔断器状态快照
type BreakerSnapshot struct {
	Kind          string    `json:"kind"` // "key" or "model"
	ID            uint      `json:"id"`
	ChannelID     uint      `json:"channel_id"`
	State         string    `json:"state"` // "cooling" / "inactive"
	FailureCount  int       `json:"failure_count"`
	LastFailure   time.Time `json:"last_failure"`
	CoolingEndsAt time.Time `json:"cooling_ends_at"`
	RemainingSec  int64     `json:"remaining_sec"`
}

// NewCircuitManager 创建熔断器管理器
func NewCircuitManager(
	db *gorm.DB,
	logger *slog.Logger,
	channelKeyRepo *repository.ChannelKeyRepository,
	modelConfigRepo *repository.ChannelModelConfigRepository,
	settingService *config.SettingService,
	failureThreshold int,
	coolingDuration time.Duration,
) *CircuitManager {
	return &CircuitManager{
		db:                  db,
		logger:              logger,
		channelKeyRepo:      channelKeyRepo,
		modelConfigRepo:     modelConfigRepo,
		settingService:      settingService,
		failureThreshold:    failureThreshold,
		coolingDuration:     coolingDuration,
		keyBreakers:         make(map[uint]*ChannelKeyBreaker),
		modelConfigBreakers: make(map[uint]*ChannelModelConfigBreaker),
	}
}

// GetKeyBreaker 获取或创建 Key 熔断器
func (m *CircuitManager) GetKeyBreaker(keyID uint) *ChannelKeyBreaker {
	m.mu.RLock()
	breaker, exists := m.keyBreakers[keyID]
	m.mu.RUnlock()

	if exists {
		return breaker
	}

	state := KeyStateActive
	failureCount := 0
	lastFailure := time.Time{}
	channelID := uint(0)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key, err := m.channelKeyRepo.FindByID(ctx, keyID)
	if err != nil {
		m.logger.Error("从数据库获取密钥状态失败",
			slog.Uint64("key_id", uint64(keyID)),
			slog.String("error", err.Error()),
		)
	} else if key != nil {
		channelID = key.ChannelID
		switch key.Status {
		case "inactive":
			state = KeyStateInactive
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

	breaker = &ChannelKeyBreaker{
		keyID:            keyID,
		channelID:        channelID,
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
func (m *CircuitManager) GetModelConfigBreaker(modelConfigID uint, channelID uint) *ChannelModelConfigBreaker {
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
		case "inactive":
			state = ModelConfigStateInactive
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if breaker, exists := m.modelConfigBreakers[modelConfigID]; exists {
		return breaker
	}

	breaker = &ChannelModelConfigBreaker{
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
func (m *CircuitManager) RecordKeySuccess(keyID uint, traceID ...string) {
	keyBreaker := m.GetKeyBreaker(keyID)
	keyBreaker.RecordSuccess()
}

// RecordKeyHardFailure 记录 Key 硬故障
func (m *CircuitManager) RecordKeyHardFailure(keyID uint, channelID uint, channelName string, errMsg string, traceID ...string) {
	keyBreaker := m.GetKeyBreaker(keyID)
	oldState := keyBreaker.state
	keyBreaker.RecordHardFailure()
	trace := normalizeTraceID(traceID...)

	m.logger.Debug("密钥硬故障",
		slog.String("trace_id", trace),
		slog.Uint64("key_id", uint64(keyID)),
		slog.Uint64("channel_id", uint64(channelID)),
		slog.String("channel_name", channelName),
		slog.String("errMsg", errMsg),
	)

	// 异步更新数据库
	if oldState != KeyStateInactive {
		go m.updateKeyStatus(keyID, "inactive")
	}

}

// RecordKeySoftFailure 记录 Key 软故障
func (m *CircuitManager) RecordKeySoftFailure(keyID uint, channelID uint, channelName string, errMsg string, traceID ...string) {
	keyBreaker := m.GetKeyBreaker(keyID)
	trace := normalizeTraceID(traceID...)

	keyBreaker.RecordSoftFailure()

	m.logger.Debug("密钥连续["+strconv.Itoa(keyBreaker.failureCount)+"]次失败",
		slog.String("trace_id", trace),
		slog.Uint64("key_id", uint64(keyID)),
		slog.Uint64("channel_id", uint64(channelID)),
		slog.String("channel_name", channelName),
	)

	// 冷却状态仅保存在内存熔断器中
	if keyBreaker.state == KeyStateCooling {
		m.logger.Debug("密钥进入冷却状态",
			slog.String("trace_id", trace),
			slog.Uint64("key_id", uint64(keyID)),
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("channel_name", channelName),
			slog.String("errMsg", errMsg),
		)
	}
}

// RecordModelConfigSuccess 记录模型配置成功
func (m *CircuitManager) RecordModelConfigSuccess(modelConfigID uint, channelID uint, traceID ...string) {
	breaker := m.GetModelConfigBreaker(modelConfigID, channelID)
	breaker.RecordSuccess()
}

// RecordModelConfigFailure 记录模型配置失败（模型映射调用失败）
func (m *CircuitManager) RecordModelConfigFailure(modelConfigID uint, channelID uint, channelName string, model string, channelModel string, errMsg string, traceID ...string) {
	breaker := m.GetModelConfigBreaker(modelConfigID, channelID)
	breaker.RecordFailure()
	trace := normalizeTraceID(traceID...)

	m.logger.Debug("模型配置连续["+strconv.Itoa(breaker.failureCount)+"]次失败",
		slog.String("trace_id", trace),
		slog.Uint64("model_config_id", uint64(modelConfigID)),
		slog.Uint64("channel_id", uint64(channelID)),
		slog.String("channel_name", channelName),
		slog.String("model", model),
		slog.String("channel_model", channelModel),
		slog.String("errMsg", errMsg),
	)

	if breaker.state == ModelConfigStateCooling {
		m.logger.Debug("模型配置进入冷却状态",
			slog.String("trace_id", trace),
			slog.Uint64("model_config_id", uint64(modelConfigID)),
			slog.Uint64("channel_id", uint64(channelID)),
			slog.String("channel_name", channelName),
			slog.String("model", model),
			slog.String("channel_model", channelModel),
		)
	}
}

func normalizeTraceID(traceID ...string) string {
	if len(traceID) == 0 {
		return "-"
	}
	value := strings.TrimSpace(traceID[0])
	if value == "" {
		return "-"
	}
	return value
}

// SnapshotBreakers 获取非 active 熔断器快照
func (m *CircuitManager) SnapshotBreakers() []BreakerSnapshot {
	now := time.Now()
	result := make([]BreakerSnapshot, 0)

	m.mu.RLock()
	for _, breaker := range m.keyBreakers {
		breaker.mu.RLock()
		state := breaker.state
		if state == KeyStateActive {
			breaker.mu.RUnlock()
			continue
		}

		snapshot := BreakerSnapshot{
			Kind:         "key",
			ID:           breaker.keyID,
			ChannelID:    breaker.channelID,
			State:        string(state),
			FailureCount: breaker.failureCount,
			LastFailure:  breaker.lastFailure,
		}
		if state == KeyStateCooling && !breaker.lastFailure.IsZero() {
			coolingWindow := breaker.coolingDuration + extraCoolingDuration(breaker.failureCount, breaker.failureThreshold)
			coolingEndsAt := breaker.lastFailure.Add(coolingWindow)
			remainingSec := remainingSeconds(now, coolingEndsAt)
			if remainingSec == 0 {
				breaker.mu.RUnlock()
				continue
			}
			snapshot.CoolingEndsAt = coolingEndsAt
			snapshot.RemainingSec = remainingSec
		}
		breaker.mu.RUnlock()
		result = append(result, snapshot)
	}

	for _, breaker := range m.modelConfigBreakers {
		breaker.mu.RLock()
		state := breaker.state
		if state == ModelConfigStateActive {
			breaker.mu.RUnlock()
			continue
		}

		snapshot := BreakerSnapshot{
			Kind:         "model",
			ID:           breaker.configID,
			ChannelID:    breaker.channelID,
			State:        string(state),
			FailureCount: breaker.failureCount,
			LastFailure:  breaker.lastFailure,
		}
		if state == ModelConfigStateCooling && !breaker.lastFailure.IsZero() {
			coolingWindow := breaker.coolingDuration + extraCoolingDuration(breaker.failureCount, breaker.failureThreshold)
			coolingEndsAt := breaker.lastFailure.Add(coolingWindow)
			remainingSec := remainingSeconds(now, coolingEndsAt)
			if remainingSec == 0 {
				breaker.mu.RUnlock()
				continue
			}
			snapshot.CoolingEndsAt = coolingEndsAt
			snapshot.RemainingSec = remainingSec
		}
		breaker.mu.RUnlock()
		result = append(result, snapshot)
	}
	m.mu.RUnlock()

	sort.Slice(result, func(i, j int) bool {
		if result[i].Kind == result[j].Kind {
			return result[i].ID < result[j].ID
		}
		return result[i].Kind < result[j].Kind
	})

	return result
}

func extraCoolingDuration(failureCount int, failureThreshold int) time.Duration {
	additionalSeconds := min(max(failureCount-failureThreshold, 0), 5) * 60
	return time.Duration(additionalSeconds) * time.Second
}

func remainingSeconds(now time.Time, endsAt time.Time) int64 {
	if endsAt.IsZero() {
		return 0
	}
	remaining := endsAt.Sub(now)
	if remaining <= 0 {
		return 0
	}
	return int64(remaining.Seconds())
}

// IsKeyAvailable 检查 Key 是否可用
func (m *CircuitManager) IsKeyAvailable(keyID uint) bool {
	breaker := m.GetKeyBreaker(keyID)
	return breaker.IsAvailable()
}

// IsModelConfigAvailable 检查模型配置是否可用
func (m *CircuitManager) IsModelConfigAvailable(modelConfigID uint) bool {
	breaker := m.GetModelConfigBreaker(modelConfigID, 0)
	return breaker.IsAvailable()
}

// Stop 停止管理器
func (m *CircuitManager) Stop() {
	// 当前熔断器管理器无后台协程，无需额外停止动作。
}

// updateKeyStatus 更新数据库中的 Key 状态
func (m *CircuitManager) updateKeyStatus(keyID uint, status string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := m.channelKeyRepo.UpdateStatus(ctx, keyID, status); err != nil {
		m.logger.Error("更新数据库中的密钥状态失败",
			slog.Uint64("key_id", uint64(keyID)),
			slog.String("status", status),
			slog.String("error", err.Error()),
		)
	}
}

// OnConfigChanged 配置变更回调（仅关注熔断器阈值与冷却时长）
func (m *CircuitManager) OnConfigChanged(ctx context.Context, category string) {
	if category != "circuit_breaker" {
		return
	}

	// 从配置服务获取最新的熔断器配置
	failureThreshold := m.settingService.GetInt(ctx, models.SettingCircuitBreakerFailureThreshold, m.failureThreshold)
	coolingDuration := m.settingService.GetDuration(ctx, models.SettingCircuitBreakerCoolingDuration, m.coolingDuration)
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

	for _, breaker := range m.modelConfigBreakers {
		breaker.UpdateConfig(failureThreshold, coolingDuration)
	}

	m.mu.Unlock()

	m.logger.Info("熔断器配置已更新",
		slog.Int("old_failure_threshold", oldFailureThreshold),
		slog.Int("new_failure_threshold", failureThreshold),
		slog.Duration("old_cooling_duration", oldCoolingDuration),
		slog.Duration("new_cooling_duration", coolingDuration),
	)
}

// RemoveKeyBreaker 移除 Key 熔断器
func (m *CircuitManager) RemoveKeyBreaker(keyID uint) {
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
func (m *CircuitManager) RemoveModelConfigBreaker(modelConfigID uint) {
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
func (m *CircuitManager) RemoveChannelBreakersAndKeys(channelID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查询该渠道下的所有密钥
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	keys, err := m.channelKeyRepo.FindByChannelID(ctx, channelID)
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
func (m *CircuitManager) CleanupOrphanBreakers(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取数据库中所有的密钥、模型配置ID（不限制状态）
	existingKeyIDs := make(map[uint]bool)
	existingModelConfigIDs := make(map[uint]bool)

	// 查询所有密钥（不限制状态）
	keys, err := m.channelKeyRepo.FindAll(ctx)
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
