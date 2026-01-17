package logger

import (
	"context"
	"log/slog"
	"strconv"
	"sync"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/service/config"
)

// DebugModeManager 调试模式管理器
type DebugModeManager struct {
	logger            *slog.Logger
	settingService    *config.SettingService
	enabled           bool
	mu                sync.RWMutex
	onChangeCallbacks []func(enabled bool)
}

// NewDebugModeManager 创建调试模式管理器
func NewDebugModeManager(
	logger *slog.Logger,
	settingService *config.SettingService,
) *DebugModeManager {
	return &DebugModeManager{
		logger:            logger,
		settingService:    settingService,
		enabled:           false,
		onChangeCallbacks: make([]func(enabled bool), 0),
	}
}

// Initialize 初始化调试模式（从系统设置读取）
func (m *DebugModeManager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 从系统设置读取调试模式配置
	debugModeStr := m.settingService.GetString(ctx, models.SettingLogDebugEnabled, "false")

	enabled, err := strconv.ParseBool(debugModeStr)
	if err != nil {
		m.logger.Warn("failed to parse debug mode from settings, using default",
			slog.String("value", debugModeStr),
			slog.String("error", err.Error()),
			slog.Bool("default", false),
		)
		m.enabled = false
		return nil
	}

	m.enabled = enabled

	m.logger.Info("debug mode initialized",
		slog.Bool("enabled", m.enabled),
	)

	return nil
}

// IsEnabled 检查调试模式是否启用
func (m *DebugModeManager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// SetEnabled 设置调试模式
func (m *DebugModeManager) SetEnabled(ctx context.Context, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 更新系统设置
	value := strconv.FormatBool(enabled)
	if err := m.settingService.Set(ctx, models.SettingLogDebugEnabled, value); err != nil {
		m.logger.Error("failed to update debug mode in settings",
			slog.Bool("enabled", enabled),
			slog.String("error", err.Error()),
		)
		return err
	}

	// 更新内存状态
	oldEnabled := m.enabled
	m.enabled = enabled

	m.logger.Info("debug mode changed",
		slog.Bool("old_enabled", oldEnabled),
		slog.Bool("new_enabled", enabled),
	)

	// 通知所有监听器
	for _, callback := range m.onChangeCallbacks {
		go callback(enabled)
	}

	return nil
}

// Toggle 切换调试模式
func (m *DebugModeManager) Toggle(ctx context.Context) (bool, error) {
	current := m.IsEnabled()
	newState := !current

	if err := m.SetEnabled(ctx, newState); err != nil {
		return current, err
	}

	return newState, nil
}

// OnChange 注册状态变化回调
func (m *DebugModeManager) OnChange(callback func(enabled bool)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onChangeCallbacks = append(m.onChangeCallbacks, callback)

	m.logger.Debug("debug mode change callback registered",
		slog.Int("total_callbacks", len(m.onChangeCallbacks)),
	)
}

// OnConfigChanged 实现 ConfigListener 接口，响应配置变更
func (m *DebugModeManager) OnConfigChanged(ctx context.Context, category string) {
	// 只处理 logging 类别的配置变更
	if category != "logging" {
		return
	}

	// 从设置服务获取最新的调试模式状态
	debugModeStr := m.settingService.GetString(ctx, models.SettingLogDebugEnabled, "false")
	enabled, err := strconv.ParseBool(debugModeStr)
	if err != nil {
		m.logger.Warn("failed to parse debug mode from config change",
			slog.String("value", debugModeStr),
			slog.String("error", err.Error()),
		)
		return
	}

	// 更新内存状态
	m.mu.Lock()
	oldEnabled := m.enabled
	m.enabled = enabled
	m.mu.Unlock()

	// 如果状态有变化，记录日志并通知所有监听器
	if oldEnabled != enabled {
		m.logger.Info("debug mode updated via config change",
			slog.Bool("old_enabled", oldEnabled),
			slog.Bool("new_enabled", enabled),
		)

		// 通知所有监听器
		for _, callback := range m.onChangeCallbacks {
			go callback(enabled)
		}
	}
}
