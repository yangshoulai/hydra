package proxy

import (
	"context"
	"sort"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
)

// ChannelSelector Channel 选择器，筛出所有可用渠道供权重路由使用
type ChannelSelector struct {
	channelRepo    *repository.ChannelRepository
	circuitManager *circuit.CircuitManager
}

// NewChannelSelector 创建 Channel 选择器
func NewChannelSelector(
	channelRepo *repository.ChannelRepository,
	circuitManager *circuit.CircuitManager,
) *ChannelSelector {
	return &ChannelSelector{
		channelRepo:    channelRepo,
		circuitManager: circuitManager,
	}
}

// SelectChannels 返回所有可用渠道（结果会按权重排序，便于日志与调试观察）
func (cs *ChannelSelector) SelectChannels(
	ctx context.Context,
	modelName string,
	endpointType string,
	isStream bool,
	excludeChannelIDs map[uint]bool,
) ([]models.Channel, error) {
	// 获取所有支持该模型和端点类型的 Channel
	channels, err := cs.channelRepo.FindByModel(ctx, modelName, endpointType, isStream)
	if err != nil {
		return nil, err
	}

	if len(channels) == 0 {
		return nil, ErrNoAvailableChannel
	}

	// 过滤出可用的 Channel
	availableChannels := cs.filterAvailableChannels(channels, excludeChannelIDs)

	if len(availableChannels) == 0 {
		return nil, ErrNoAvailableChannel
	}

	cs.sortChannelsByWeight(availableChannels)
	return availableChannels, nil
}

// filterAvailableChannels 过滤出可用的 Channel
func (cs *ChannelSelector) filterAvailableChannels(channels []models.Channel, excludeChannelIDs map[uint]bool) []models.Channel {
	available := make([]models.Channel, 0, len(channels))
	for _, channel := range channels {
		if excludeChannelIDs[channel.ID] {
			continue
		}
		var filteredModelConfigs []models.ChannelModelConfig
		var filteredKeys []models.ChannelKey
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
		for _, key := range channel.ChannelKeys {
			if !cs.circuitManager.IsKeyAvailable(key.ID) {
				continue
			}
			if _, ok := modelKeyGroups[key.ChannelKeyGroup]; ok {
				filteredKeys = append(filteredKeys, key)
			}
		}

		if len(filteredModelConfigs) == 0 || len(filteredKeys) == 0 {
			continue
		}
		channel.ModelConfigs = filteredModelConfigs
		channel.ChannelKeys = filteredKeys
		available = append(available, channel)
	}
	return available
}

// sortChannelsByWeight 渠道按权重排序（高权重优先）
func (cs *ChannelSelector) sortChannelsByWeight(channels []models.Channel) {
	sort.SliceStable(channels, func(i, j int) bool {
		if channels[i].Weight == channels[j].Weight {
			return channels[i].ID > channels[j].ID
		}
		return channels[i].Weight > channels[j].Weight
	})
}
