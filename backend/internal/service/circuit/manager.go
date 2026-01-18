package circuit

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
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
	settingService   *config.SettingService // 配置服务，用于获取最新配置
	failureThreshold int
	coolingDuration  time.Duration

	keyBreakers     map[uint]*KeyBreaker
	channelBreakers map[uint]*ChannelBreaker

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
	settingService *config.SettingService,
	failureThreshold int,
	coolingDuration time.Duration,
	probeInterval time.Duration,
) *Manager {
	return &Manager{
		db:                 db,
		logger:             logger,
		keyRepo:            keyRepo,
		channelRepo:        channelRepo,
		settingService:     settingService,
		failureThreshold:   failureThreshold,
		coolingDuration:    coolingDuration,
		probeInterval:      probeInterval,
		probeMaxConcurrent: 10, // 默认值
		keyBreakers:        make(map[uint]*KeyBreaker),
		channelBreakers:    make(map[uint]*ChannelBreaker),
		stopChan:           make(chan struct{}),
		restartChan:        make(chan struct{}),
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

	m.logger.Info("创建密钥熔断器",
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

	m.logger.Info("创建渠道熔断器",
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

	// 异步更新数据库，如果之前在 cooling 状态，需要清除 cooling_at
	go m.exitKeyCooling(keyID)
}

// RecordKeyHardFailure 记录 Key 硬故障
func (m *Manager) RecordKeyHardFailure(keyID uint, channelID uint) {
	keyBreaker := m.GetKeyBreaker(keyID)
	keyBreaker.RecordHardFailure()

	m.logger.Warn("密钥硬故障",
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
		m.logger.Warn("密钥进入冷却状态",
			slog.Uint64("key_id", uint64(keyID)),
			slog.Uint64("channel_id", uint64(channelID)),
		)
		go m.enterKeyCooling(keyID)
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

	m.logger.Info("重置密钥熔断器",
		slog.Uint64("key_id", uint64(keyID)),
	)

	// 更新数据库，清除 cooling_at
	go m.exitKeyCooling(keyID)
}

// StartProbeScheduler 启动探测调度器
func (m *Manager) StartProbeScheduler() {
	m.logger.Info("熔断器探测调度器启动中")

	for {
		m.mu.RLock()
		interval := m.probeInterval
		m.mu.RUnlock()

		ticker := time.NewTicker(interval)

		m.logger.Info("熔断器探测调度器已启动",
			slog.Duration("interval", interval),
		)

	loop:
		for {
			select {
			case <-ticker.C:
				m.probeHalfOpenKeys()
			case <-m.restartChan:
				ticker.Stop()
				m.logger.Info("熔断器探测调度器因配置变更而重启")
				break loop
			case <-m.stopChan:
				ticker.Stop()
				m.logger.Info("熔断器探测调度器已停止")
				return
			}
		}
	}
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

// probeHalfOpenKeys 探测半开状态的 Key
func (m *Manager) probeHalfOpenKeys() {
	// 检查是否已有探测在进行中
	if !atomic.CompareAndSwapInt32(&m.probing, 0, 1) {
		m.logger.Warn("跳过探测周期，上一次探测仍在进行中")
		return
	}

	// 获取当前配置的最大并发探测数
	m.mu.RLock()
	maxConcurrentProbes := m.probeMaxConcurrent
	m.mu.RUnlock()

	m.mu.RLock()
	candidates := make([]keyProbeCandidate, 0)
	for keyID, breaker := range m.keyBreakers {
		if breaker.GetState() == KeyStateHalfOpen {
			stats := breaker.GetStats()
			if lastFailure, ok := stats["last_failure"].(time.Time); ok {
				candidates = append(candidates, keyProbeCandidate{
					keyID:       keyID,
					lastFailure: lastFailure,
				})
			}
		}
	}
	m.mu.RUnlock()

	if len(candidates) == 0 {
		atomic.StoreInt32(&m.probing, 0)
		return
	}

	// 按冷却时间排序，优先探测冷却时间最长的 Key
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].lastFailure.Before(candidates[j].lastFailure)
	})

	// 限制并发探测数量
	probeCount := len(candidates)
	if probeCount > maxConcurrentProbes {
		probeCount = maxConcurrentProbes
	}

	m.logger.Info("探测半开状态的密钥",
		slog.Int("total", len(candidates)),
		slog.Int("probing", probeCount),
		slog.Int("max_concurrent", maxConcurrentProbes),
	)

	// 使用 WaitGroup 等待所有探测完成
	var wg sync.WaitGroup
	wg.Add(probeCount)

	// 探测优先级最高的 N 个 Key
	for i := 0; i < probeCount; i++ {
		go func(keyID uint) {
			defer wg.Done()
			m.probeKey(keyID)
		}(candidates[i].keyID)
	}

	// 在后台等待所有探测完成后清除标志
	go func() {
		wg.Wait()
		atomic.StoreInt32(&m.probing, 0)
		m.logger.Debug("探测周期完成")
	}()
}

// probeKey 探测单个 Key
func (m *Manager) probeKey(keyID uint) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 获取 Key 信息
	key, err := m.keyRepo.FindByID(ctx, keyID)
	if err != nil {
		m.logger.Error("探测时查找密钥失败",
			slog.Uint64("key_id", uint64(keyID)),
			slog.String("error", err.Error()),
		)
		return
	}

	// 获取 Channel 信息
	channel, err := m.channelRepo.FindByID(ctx, key.ChannelID)
	if err != nil {
		m.logger.Error("探测时查找渠道失败",
			slog.Uint64("key_id", uint64(keyID)),
			slog.Uint64("channel_id", uint64(key.ChannelID)),
			slog.String("error", err.Error()),
		)
		return
	}

	// 创建探测处理器
	probeHandler := NewProbeHandler(m, m.logger)

	// 执行探测(带重试)
	success, isHardFailure, err := probeHandler.ProbeKeyWithRetry(ctx, key, channel, 2)

	// 处理探测结果
	probeHandler.HandleProbeResult(keyID, key.ChannelID, success, isHardFailure)

	if err != nil {
		m.logger.Debug("探测完成但有错误",
			slog.Uint64("key_id", uint64(keyID)),
			slog.Bool("success", success),
			slog.String("error", err.Error()),
		)
	}
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
	for _, breaker := range m.channelBreakers {
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
