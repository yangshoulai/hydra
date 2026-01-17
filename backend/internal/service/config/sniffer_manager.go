package config

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/yangshoulai/hydra/internal/models"
)

// SnifferManager 嗅探器配置管理器（单例）
var snifferManagerInstance *SnifferManager
var snifferManagerOnce sync.Once

// SnifferUpdater 嗅探器更新器接口
type SnifferUpdater interface {
	UpdateSnifferKeywords(keywords []string)
}

// SnifferManager 嗅探器配置管理器
type SnifferManager struct {
	logger    *slog.Logger
	updater   SnifferUpdater
	mu        sync.RWMutex
	initialized bool
	settingService *SettingService
}

// GetSnifferManager 获取嗅探器配置管理器单例
func GetSnifferManager() *SnifferManager {
	if snifferManagerInstance == nil {
		snifferManagerOnce.Do(func() {
			snifferManagerInstance = &SnifferManager{
				logger:      slog.Default(),
				initialized: false,
			}
		})
	}
	return snifferManagerInstance
}

// SetSettingService 设置系统设置服务
func (m *SnifferManager) SetSettingService(settingService *SettingService) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settingService = settingService
}

// RegisterUpdater 注册嗅探器更新器
func (m *SnifferManager) RegisterUpdater(updater SnifferUpdater) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updater = updater
	m.initialized = true
}

// UpdateKeywords 更新关键词
func (m *SnifferManager) UpdateKeywords(ctx context.Context, keywords []string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.updater == nil {
		slog.Default().Warn("sniffer updater not registered, cannot update keywords")
		return
	}

	m.updater.UpdateSnifferKeywords(keywords)
	slog.Default().Info("sniffer keywords updated via manager",
		"count", len(keywords),
	)
}

// IsInitialized 检查是否已初始化
func (m *SnifferManager) IsInitialized() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialized
}

// OnConfigChanged 配置变更监听器实现
func (m *SnifferManager) OnConfigChanged(ctx context.Context, category string) {
	// 只处理 sniffer 类别的配置变更
	if category != "sniffer" {
		return
	}

	m.mu.RLock()
	settingService := m.settingService
	m.mu.RUnlock()

	if settingService == nil {
		slog.Default().Warn("settingService not set in sniffer manager")
		return
	}

	// 获取最新的关键词
	keywordsJSON := settingService.GetString(ctx, models.SettingSnifferPlainTextErrorRules, "[]")
	var keywords []string
	if err := json.Unmarshal([]byte(keywordsJSON), &keywords); err != nil {
		slog.Default().Error("failed to unmarshal sniffer keywords",
			"error", err.Error(),
		)
		return
	}

	// 更新关键词
	m.UpdateKeywords(ctx, keywords)
	slog.Default().Info("sniffer keywords updated via config listener",
		"count", len(keywords),
	)
}
