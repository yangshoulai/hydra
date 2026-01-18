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
		cs.logger.Error("查找支持模型的渠道失败",
			slog.String("trace_id", traceID),
			slog.String("model", modelName),
			slog.String("endpoint_type", endpointType),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if len(channels) == 0 {
		cs.logger.Warn("没有渠道支持该模型",
			slog.String("trace_id", traceID),
			slog.String("model", modelName),
			slog.String("endpoint_type", endpointType),
		)
		return nil, ErrNoAvailableChannel
	}

	// 过滤出可用的 Channel
	availableChannels := cs.filterAvailableChannels(channels, traceID)

	if len(availableChannels) == 0 {
		cs.logger.Warn("no available channels for model",
			slog.String("trace_id", traceID),
			slog.String("model", modelName),
			slog.Int("total_channels", len(channels)),
		)
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
			cs.logger.Debug("channel selected",
				slog.String("trace_id", traceID),
				slog.Uint64("channel_id", uint64(selectedChannel.ID)),
				slog.String("channel_name", selectedChannel.Name),
				slog.Int("priority", selectedChannel.Priority),
				slog.Int("weight", selectedChannel.Weight),
			)
			return selectedChannel, nil
		}
	}

	cs.logger.Warn("all channels are unavailable",
		slog.String("trace_id", traceID),
		slog.String("model", modelName),
	)
	return nil, ErrNoAvailableChannel
}

// filterAvailableChannels 过滤出可用的 Channel
func (cs *ChannelSelector) filterAvailableChannels(channels []models.Channel, traceID string) []models.Channel {
	available := make([]models.Channel, 0, len(channels))

	for _, channel := range channels {
		// 检查 Channel 是否激活
		if !channel.IsActive() {
			cs.logger.Debug("channel is not active",
				slog.String("trace_id", traceID),
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.String("status", channel.Status),
			)
			continue
		}

		// 检查熔断器状态
		if !cs.circuitManager.IsChannelAvailable(channel.ID) {
			cs.logger.Debug("channel is not available (circuit breaker)",
				slog.String("trace_id", traceID),
				slog.Uint64("channel_id", uint64(channel.ID)),
			)
			continue
		}

		// 检查是否有可用的 Key
		if cs.keySelector.GetAvailableKeyCount(&channel, traceID) == 0 {
			cs.logger.Debug("channel has no available keys",
				slog.String("trace_id", traceID),
				slog.Uint64("channel_id", uint64(channel.ID)),
			)
			continue
		}

		available = append(available, channel)
	}

	return available
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

	available := cs.filterAvailableChannels(channels, "")
	return len(available), nil
}
