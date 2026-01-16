package config

import (
	"context"
	"log/slog"
	"sync"
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
