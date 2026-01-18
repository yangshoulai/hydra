package proxy

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/service/circuit"
)

var (
	// ErrNoAvailableKey 无可用 Key
	ErrNoAvailableKey = errors.New("no available key in pool")
)

// KeySelector Key 选择器,从可用 Key 池中轮询选择
type KeySelector struct {
	mu             sync.Mutex
	logger         *slog.Logger
	circuitManager *circuit.Manager
	// 轮询计数器,每个 Channel 维护独立的计数器
	roundRobinCounters map[uint]int
}

// NewKeySelector 创建 Key 选择器
func NewKeySelector(logger *slog.Logger, circuitManager *circuit.Manager) *KeySelector {
	return &KeySelector{
		logger:             logger,
		circuitManager:     circuitManager,
		roundRobinCounters: make(map[uint]int),
	}
}

// SelectKey 从 Channel 的 Key 池中选择一个可用的 Key
// 使用轮询(Round Robin)策略
func (ks *KeySelector) SelectKey(channel *models.Channel, traceID string) (*models.Key, error) {
	if channel == nil {
		return nil, errors.New("channel is nil")
	}

	if len(channel.Keys) == 0 {
		ks.logger.Warn("channel has no keys",
			slog.String("trace_id", traceID),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
		)
		return nil, ErrNoAvailableKey
	}

	// 获取所有可用的 Key
	availableKeys := ks.getAvailableKeys(channel, traceID)

	if len(availableKeys) == 0 {
		ks.logger.Warn("no available keys in channel",
			slog.String("trace_id", traceID),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.Int("total_keys", len(channel.Keys)),
		)
		return nil, ErrNoAvailableKey
	}

	// 使用轮询策略选择 Key
	selectedKey := ks.selectByRoundRobin(channel.ID, availableKeys)

	ks.logger.Debug("key selected",
		slog.String("trace_id", traceID),
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("channel_name", channel.Name),
		slog.Uint64("key_id", uint64(selectedKey.ID)),
		slog.Int("available_keys", len(availableKeys)),
	)

	return selectedKey, nil
}

// getAvailableKeys 获取所有可用的 Key
func (ks *KeySelector) getAvailableKeys(channel *models.Channel, traceID string) []*models.Key {
	availableKeys := make([]*models.Key, 0, len(channel.Keys))

	for i := range channel.Keys {
		key := &channel.Keys[i]

		// 检查 Key 是否被禁用
		if key.Status == "disabled" {
			ks.logger.Debug("key is disabled",
				slog.String("trace_id", traceID),
				slog.Uint64("key_id", uint64(key.ID)),
			)
			continue
		}

		// 检查熔断器状态
		if !ks.circuitManager.IsKeyAvailable(key.ID) {
			ks.logger.Debug("key is not available (circuit breaker)",
				slog.String("trace_id", traceID),
				slog.Uint64("key_id", uint64(key.ID)),
			)
			continue
		}

		availableKeys = append(availableKeys, key)
	}

	return availableKeys
}

// selectByRoundRobin 使用轮询策略选择 Key
func (ks *KeySelector) selectByRoundRobin(channelID uint, availableKeys []*models.Key) *models.Key {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	// 获取当前计数器值
	counter, exists := ks.roundRobinCounters[channelID]
	if !exists {
		counter = 0
	}

	// 选择 Key
	index := counter % len(availableKeys)
	selectedKey := availableKeys[index]

	// 更新计数器
	ks.roundRobinCounters[channelID] = counter + 1

	// 防止计数器无限增长
	if ks.roundRobinCounters[channelID] > 1000000 {
		ks.roundRobinCounters[channelID] = 0
	}

	return selectedKey
}

// ResetCounter 重置指定 Channel 的轮询计数器
func (ks *KeySelector) ResetCounter(channelID uint) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	delete(ks.roundRobinCounters, channelID)
}

// GetAvailableKeyCount 获取指定 Channel 的可用 Key 数量
func (ks *KeySelector) GetAvailableKeyCount(channel *models.Channel, traceID string) int {
	if channel == nil {
		return 0
	}
	return len(ks.getAvailableKeys(channel, traceID))
}
