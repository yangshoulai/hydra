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
func (cs *ChannelSelector) SelectChannel(ctx context.Context, modelName string, endpointType string, traceID string) (*models.Channel, error) {
	// 获取所有支持该模型和端点类型的 Channel
	channels, err := cs.channelRepo.FindByModelAndEndpointType(ctx, modelName, endpointType)
	if err != nil {
		return nil, err
	}

	if len(channels) == 0 {
		return nil, ErrNoAvailableChannel
	}

	// 过滤出可用的 Channel
	availableChannels := cs.filterAvailableChannels(channels, modelName, endpointType, traceID)

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
func (cs *ChannelSelector) filterAvailableChannels(channels []models.Channel, modelName string, endpointType string, traceID string) []models.Channel {
	available := make([]models.Channel, 0, len(channels))

	for _, channel := range channels {
		// 检查 Channel 是否激活
		if !channel.IsActive() {
			cs.logger.Debug("渠道处于非正常状态",
				slog.String("trace_id", traceID),
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.String("status", channel.Status),
			)
			continue
		}

		// 检查模型配置熔断状态与密钥分组可用性
		if cs.getAvailableModelConfig(&channel, modelName, endpointType, traceID) == nil {
			cs.logger.Debug("渠道没有可用模型配置",
				slog.String("trace_id", traceID),
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.String("unified_model", modelName),
			)
			continue
		}

		available = append(available, channel)
	}

	return available
}

// getAvailableModelConfig 获取渠道可用的模型配置
func (cs *ChannelSelector) getAvailableModelConfig(channel *models.Channel, unifiedModel string, endpointType string, traceID string) *models.ChannelModelConfig {
	if channel == nil {
		return nil
	}

	for i := range channel.ModelConfigs {
		config := &channel.ModelConfigs[i]
		if config.UnifiedModel != unifiedModel || !config.IsActive() {
			continue
		}
		if !cs.hasEndpointType(config.EndpointTypes, endpointType) {
			continue
		}
		if cs.circuitManager != nil && !cs.circuitManager.IsModelConfigAvailable(config.ID) {
			cs.logger.Debug("model config is cooling",
				slog.String("trace_id", traceID),
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.Uint64("model_config_id", uint64(config.ID)),
				slog.String("unified_model", unifiedModel),
				slog.String("upstream_model", config.UpstreamModel),
			)
			continue
		}

		if cs.keySelector.GetAvailableKeyCount(channel, config.KeyGroups, traceID) == 0 {
			cs.logger.Debug("模型配置没有可用密钥",
				slog.String("trace_id", traceID),
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.Uint64("model_config_id", uint64(config.ID)),
				slog.String("unified_model", unifiedModel),
				slog.String("upstream_model", config.UpstreamModel),
			)
			continue
		}

		return config
	}

	return nil
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

// GetAvailableChannelCount 获取可用 Channel 数量
func (cs *ChannelSelector) GetAvailableChannelCount(ctx context.Context, modelName string) (int, error) {
	channels, err := cs.channelRepo.FindByModel(ctx, modelName)
	if err != nil {
		return 0, err
	}

	available := cs.filterAvailableChannels(channels, modelName, "", "")
	return len(available), nil
}
