package proxy

import (
	"log/slog"
	"sync"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/service/circuit"
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
func (ks *KeySelector) SelectKey(channel *models.Channel, availableKeys []models.Key) models.Key {
	// 使用轮询策略选择 Key
	selectedKey := ks.selectByRoundRobin(channel.ID, availableKeys)
	return selectedKey
}

// selectByRoundRobin 使用轮询策略选择 Key
func (ks *KeySelector) selectByRoundRobin(channelID uint, availableKeys []models.Key) models.Key {
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
