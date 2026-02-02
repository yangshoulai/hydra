package proxy

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"sort"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
)

var (
	// ErrNoAvailableChannel 无可用渠道
	ErrNoAvailableChannel = errors.New("no available channel")
)

// ChannelSelector Channel 选择器,按优先级和权重选择
type ChannelSelector struct {
	logger         *slog.Logger
	channelRepo    *repository.ChannelRepository
	keySelector    *KeySelector
	circuitManager *circuit.Manager
}

// NewChannelSelector 创建 Channel 选择器
func NewChannelSelector(
	logger *slog.Logger,
	channelRepo *repository.ChannelRepository,
	keySelector *KeySelector,
	circuitManager *circuit.Manager,
) *ChannelSelector {
	return &ChannelSelector{
		logger:         logger,
		channelRepo:    channelRepo,
		keySelector:    keySelector,
		circuitManager: circuitManager,
	}
}

// SelectChannel 按优先级和权重选择一个可用的 Channel
// excludeChannelIDs: 需要排除的渠道集合（可为空）
func (cs *ChannelSelector) SelectChannel(ctx context.Context, modelName string, endpointType string, traceID string, excludeChannelIDs map[uint]bool) (*models.Channel, error) {
	// 获取所有支持该模型和端点类型的 Channel
	channels, err := cs.channelRepo.FindByModel(ctx, modelName, endpointType)
	if err != nil {
		return nil, err
	}

	if len(channels) == 0 {
		return nil, ErrNoAvailableChannel
	}

	// 过滤出可用的 Channel
	availableChannels := cs.filterAvailableChannels(channels, modelName, endpointType, traceID, excludeChannelIDs)

	if len(availableChannels) == 0 {
		return nil, ErrNoAvailableChannel
	}

	// 按优先级分组
	channelGroups := cs.groupByPriority(availableChannels)

	// 按优先级从高到低尝试选择
	priorities := make([]int, 0, len(channelGroups))
	for priority := range channelGroups {
		priorities = append(priorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(priorities)))

	for _, priority := range priorities {
		channelsInGroup := channelGroups[priority]
		// 按权重选择
		selectedChannel := cs.selectByWeight(channelsInGroup)
		if selectedChannel != nil {
			return selectedChannel, nil
		}
	}
	return nil, ErrNoAvailableChannel
}

// filterAvailableChannels 过滤出可用的 Channel
func (cs *ChannelSelector) filterAvailableChannels(channels []models.Channel, modelName string, endpointType string, traceID string, excludeChannelIDs map[uint]bool) []models.Channel {
	available := make([]models.Channel, 0, len(channels))
	for _, channel := range channels {
		if excludeChannelIDs != nil && excludeChannelIDs[channel.ID] {
			continue
		}
		var filteredModelConfigs []models.ChannelModelConfig
		var filteredKeys []models.Key
		modelKeyGroups := make(map[string]struct{})

		// 过滤不支持端点类型的模型
		for _, config := range channel.ModelConfigs {
			if !cs.circuitManager.IsModelConfigAvailable(config.ID) {
				continue
			}

			filteredModelConfigs = append(filteredModelConfigs, config)
			for _, kg := range config.KeyGroups {
				modelKeyGroups[kg] = struct{}{}
			}
		}

		// 过滤不在密钥分组的渠道密钥
		for _, key := range channel.Keys {
			if !cs.circuitManager.IsKeyAvailable(key.ID) {
				continue
			}
			if _, ok := modelKeyGroups[key.KeyGroup]; ok {
				filteredKeys = append(filteredKeys, key)
			}
		}

		if len(filteredModelConfigs) == 0 || len(filteredKeys) == 0 {
			continue
		}
		channel.ModelConfigs = filteredModelConfigs
		channel.Keys = filteredKeys
		available = append(available, channel)
	}
	return available
}

// hasEndpointType 检查端点类型是否匹配
func (cs *ChannelSelector) hasEndpointType(endpointTypes []string, targetType string) bool {
	if targetType == "" {
		return true
	}
	for _, et := range endpointTypes {
		if et == targetType {
			return true
		}
	}
	return false
}

// groupByPriority 按优先级分组
func (cs *ChannelSelector) groupByPriority(channels []models.Channel) map[int][]models.Channel {
	groups := make(map[int][]models.Channel)

	for _, channel := range channels {
		priority := channel.Priority
		groups[priority] = append(groups[priority], channel)
	}

	return groups
}

// selectByWeight 按权重选择 Channel
func (cs *ChannelSelector) selectByWeight(channels []models.Channel) *models.Channel {
	if len(channels) == 0 {
		return nil
	}

	if len(channels) == 1 {
		return &channels[0]
	}

	// 计算总权重
	totalWeight := 0
	for _, channel := range channels {
		if channel.Weight <= 0 {
			channel.Weight = 1
		}
		totalWeight += channel.Weight
	}

	// 生成随机数
	randomWeight := rand.Intn(totalWeight)

	// 加权随机选择
	currentWeight := 0
	for i := range channels {
		currentWeight += channels[i].Weight
		if randomWeight < currentWeight {
			return &channels[i]
		}
	}

	// 兜底,返回最后一个
	return &channels[len(channels)-1]
}
