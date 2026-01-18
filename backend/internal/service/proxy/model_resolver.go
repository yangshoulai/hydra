package proxy

import (
	"context"
	"log/slog"
	"sort"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
)

// ModelResolutionResult 模型解析结果
type ModelResolutionResult struct {
	Channel       *models.Channel // 选中的渠道
	UpstreamModel string          // 上游模型名
	Weight        int             // 选中渠道的权重
	Priority      int             // 选中渠道的优先级
}

// ModelResolver 模型解析器,处理模型名映射冲突
// 当多个渠道都支持同一个统一模型名时,按权重和优先级选择最佳渠道
type ModelResolver struct {
	logger      *slog.Logger
	channelRepo *repository.ChannelRepository
	modelRouter *ModelRouter
}

// NewModelResolver 创建模型解析器
func NewModelResolver(
	logger *slog.Logger,
	channelRepo *repository.ChannelRepository,
	modelRouter *ModelRouter,
) *ModelResolver {
	return &ModelResolver{
		logger:      logger,
		channelRepo: channelRepo,
		modelRouter: modelRouter,
	}
}

// ResolveModelMapping 解析模型映射冲突
// 返回所有支持该模型的渠道及其上游模型名,按优先级和权重排序
func (mr *ModelResolver) ResolveModelMapping(ctx context.Context, unifiedModel string) ([]ModelResolutionResult, error) {
	// 获取所有支持该模型的渠道
	channels, err := mr.channelRepo.FindByModel(ctx, unifiedModel)
	if err != nil {
		mr.logger.Error("failed to find channels by model",
			slog.String("model", unifiedModel),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if len(channels) == 0 {
		mr.logger.Warn("no channels support unified model",
			slog.String("model", unifiedModel),
		)
		return []ModelResolutionResult{}, nil
	}

	// 构建解析结果
	results := make([]ModelResolutionResult, 0, len(channels))
	for _, channel := range channels {
		// 只考虑激活的渠道
		if !channel.IsActive() {
			continue
		}

		// 获取上游模型名（传空字符串表示匹配所有端点类型）
		upstreamModel, err := mr.modelRouter.RouteModel(unifiedModel, &channel, "", "")
		if err != nil {
			mr.logger.Debug("failed to route model for channel",
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.String("model", unifiedModel),
				slog.String("error", err.Error()),
			)
			continue
		}

		results = append(results, ModelResolutionResult{
			Channel:       &channel,
			UpstreamModel: upstreamModel,
			Weight:        channel.Weight,
			Priority:      channel.Priority,
		})
	}

	// 按优先级(降序)和权重(降序)排序
	sort.Slice(results, func(i, j int) bool {
		// 优先级高的在前
		if results[i].Priority != results[j].Priority {
			return results[i].Priority > results[j].Priority
		}
		// 优先级相同时,权重高的在前
		return results[i].Weight > results[j].Weight
	})

	if len(results) > 1 {
		mr.logger.Info("multiple channels support unified model",
			slog.String("model", unifiedModel),
			slog.Int("count", len(results)),
			slog.Uint64("preferred_channel_id", uint64(results[0].Channel.ID)),
			slog.String("preferred_channel_name", results[0].Channel.Name),
		)
	}

	return results, nil
}

// GetPreferredChannel 获取首选渠道
// 根据权重和优先级,返回支持该模型的最佳渠道
func (mr *ModelResolver) GetPreferredChannel(ctx context.Context, unifiedModel string) (*ModelResolutionResult, error) {
	results, err := mr.ResolveModelMapping(ctx, unifiedModel)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		mr.logger.Warn("no channels available for model",
			slog.String("model", unifiedModel),
		)
		return nil, ErrNoAvailableChannel
	}

	// 返回排序后的第一个(优先级和权重最高的)
	preferred := results[0]
	
	mr.logger.Debug("preferred channel selected for model",
		slog.String("model", unifiedModel),
		slog.Uint64("channel_id", uint64(preferred.Channel.ID)),
		slog.String("channel_name", preferred.Channel.Name),
		slog.Int("priority", preferred.Priority),
		slog.Int("weight", preferred.Weight),
		slog.String("upstream_model", preferred.UpstreamModel),
	)

	return &preferred, nil
}

// GetAllSupportedModels 获取所有支持的统一模型名
// 返回去重后的模型列表及每个模型支持的渠道数量
func (mr *ModelResolver) GetAllSupportedModels(ctx context.Context) (map[string]int, error) {
	// 获取所有激活的渠道
	channels, err := mr.channelRepo.FindAll(ctx)
	if err != nil {
		mr.logger.Error("failed to find all channels",
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	// 统计每个模型支持的渠道数量
	modelCounts := make(map[string]int)
	for _, channel := range channels {
		if !channel.IsActive() {
			continue
		}

		supportedModels := mr.modelRouter.GetSupportedModels(channel)
		for _, model := range supportedModels {
			modelCounts[model]++
		}
	}

	mr.logger.Debug("aggregated supported models",
		slog.Int("total_unique_models", len(modelCounts)),
	)

	return modelCounts, nil
}

// DetectModelConflicts 检测模型映射冲突
// 返回有多个渠道支持的模型及其渠道列表
func (mr *ModelResolver) DetectModelConflicts(ctx context.Context) (map[string][]string, error) {
	channels, err := mr.channelRepo.FindAll(ctx)
	if err != nil {
		mr.logger.Error("failed to find all channels",
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	// 记录每个模型支持的渠道
	modelChannels := make(map[string][]string)
	for _, channel := range channels {
		if !channel.IsActive() {
			continue
		}

		supportedModels := mr.modelRouter.GetSupportedModels(channel)
		for _, model := range supportedModels {
			modelChannels[model] = append(modelChannels[model], channel.Name)
		}
	}

	// 只保留有冲突的(多个渠道支持的)
	conflicts := make(map[string][]string)
	for model, channelNames := range modelChannels {
		if len(channelNames) > 1 {
			conflicts[model] = channelNames
		}
	}

	if len(conflicts) > 0 {
		mr.logger.Info("detected model mapping conflicts",
			slog.Int("conflicting_models", len(conflicts)),
		)
	}

	return conflicts, nil
}
