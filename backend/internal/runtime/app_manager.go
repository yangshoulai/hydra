package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultRestartMergeWindow = 300 * time.Millisecond
	defaultStopTimeout        = 10 * time.Second
)

// AppManager 负责管理 App 生命周期（启动、重启、优雅停止）
//
// 关键点：
// 1. 当需要重启的系统设置变更时，通过 OnConfigChanged 触发重启。
// 2. 对重启请求做合并，避免短时间内重复重启。
// 3. 重启流程：先优雅停止旧实例，再启动新实例（允许短暂中断）。
type AppManager struct {
	logger  *slog.Logger
	dataDir string

	mu      sync.RWMutex
	current *App
	nextID  int64

	restartCh    chan string
	shutdownCh   chan struct{}
	loopRunning  atomic.Bool
	restarting   atomic.Bool
	shutdownOnce sync.Once
}

var restartRequiredCategories = map[string]struct{}{
	"server":   {},
	"security": {},
	"unknown":  {},
}

// NewAppManager 创建 AppManager。
func NewAppManager(logger *slog.Logger, dataDir string) *AppManager {
	return &AppManager{
		logger:     logger,
		dataDir:    dataDir,
		restartCh:  make(chan string, 1),
		shutdownCh: make(chan struct{}),
	}
}

// Start 启动首个 App，并启动重启调度循环
func (m *AppManager) Start() error {
	if !m.loopRunning.CompareAndSwap(false, true) {
		return nil
	}

	if err := m.startNewApp("initial_start"); err != nil {
		m.loopRunning.Store(false)
		return err
	}

	go m.loop()
	return nil
}

func (m *AppManager) loop() {
	for {
		select {
		case reason := <-m.restartCh:
			reason = normalizeReason(reason)

			// 短暂合并窗口，避免一次批量设置触发多次重启
			mergeDeadline := time.NewTimer(defaultRestartMergeWindow)
			for {
				select {
				case moreReason := <-m.restartCh:
					reason = normalizeReason(moreReason)
				case <-mergeDeadline.C:
					goto restart
				}
			}
		restart:
			if !mergeDeadline.Stop() {
				select {
				case <-mergeDeadline.C:
				default:
				}
			}
			if err := m.restart(reason); err != nil {
				m.logger.Error("应用重启失败", slog.String("reason", reason), slog.String("error", err.Error()))
			}
		case <-m.shutdownCh:
			m.loopRunning.Store(false)
			return
		}
	}
}

func (m *AppManager) restart(reason string) error {
	if !m.restarting.CompareAndSwap(false, true) {
		m.logger.Warn("已有重启流程在执行，忽略重复请求", slog.String("reason", reason))
		return nil
	}
	defer m.restarting.Store(false)

	m.logger.Info("开始重启应用", slog.String("reason", reason))

	m.mu.RLock()
	oldApp := m.current
	m.mu.RUnlock()

	if oldApp != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), defaultStopTimeout)
		_ = oldApp.Stop(stopCtx)
		cancel()
	}

	if err := m.startNewApp(reason); err != nil {
		return err
	}

	m.logger.Info("应用重启完成", slog.String("reason", reason))
	return nil
}

func (m *AppManager) startNewApp(reason string) error {
	id := atomic.AddInt64(&m.nextID, 1)
	newApp, err := NewApp(id, m.dataDir, m.logger, m)
	if err != nil {
		return fmt.Errorf("创建新 App 失败: %w", err)
	}
	if err := newApp.Start(); err != nil {
		return fmt.Errorf("启动新 App 失败: %w", err)
	}

	m.mu.Lock()
	m.current = newApp
	m.mu.Unlock()

	m.logger.Info("新 App 实例已启动", slog.Int64("app_id", id), slog.String("reason", reason))
	return nil
}

// RequestRestart 请求重启
func (m *AppManager) RequestRestart(reason string) {
	select {
	case m.restartCh <- normalizeReason(reason):
	default:
		// 通道满表示已有重启请求在排队，忽略即可
	}
}

// OnConfigChanged 实现 ConfigListener，配置变更时重启 App
func (m *AppManager) OnConfigChanged(_ context.Context, category string) {
	if _, requiresRestart := restartRequiredCategories[category]; !requiresRestart {
		m.logger.Info("配置已热更新，无需重启应用",
			slog.String("category", category),
		)
		return
	}
	m.RequestRestart("config_changed:" + category)
}

// Shutdown 停止管理器和当前 App
func (m *AppManager) Shutdown(ctx context.Context) error {
	m.shutdownOnce.Do(func() {
		close(m.shutdownCh)
	})

	m.mu.RLock()
	current := m.current
	m.mu.RUnlock()

	if current == nil {
		return nil
	}
	return current.Stop(ctx)
}

func normalizeReason(reason string) string {
	if reason == "" {
		return "manual_restart"
	}
	return reason
}
