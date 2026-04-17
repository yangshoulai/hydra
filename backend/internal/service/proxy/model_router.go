package proxy

import (
	"log/slog"

	"github.com/yangshoulai/hydra/internal/models"
)

// ModelRouter 模型路由器，负责模型名到渠道模型名的映射
type ModelRouter struct {
	logger *slog.Logger
}

// NewModelRouter 创建模型路由器
func NewModelRouter(logger *slog.Logger) *ModelRouter {
	return &ModelRouter{
		logger: logger,
	}
}

// RouteModel 在选定渠道内按权重路由到渠道模型名
// channel: 选定的渠道
// availableModelConfigs: 可用渠道模型
// traceID: 请求追踪ID
// 返回: 匹配的模型配置
func (mr *ModelRouter) RouteModel(channel *models.Channel, availableModelConfigs []models.ChannelModelConfig, traceID string) models.ChannelModelConfig {
	weights := make([]int64, 0, len(availableModelConfigs))
	for _, config := range availableModelConfigs {
		weights = append(weights, modelConfigWeightWithFallback(config, channelWeight(channel)))
	}

	selectedIndex := weightedRandomIndex(weights)
	if selectedIndex < 0 || selectedIndex >= len(availableModelConfigs) {
		selectedIndex = 0
	}
	selectedConfig := availableModelConfigs[selectedIndex]

	if len(availableModelConfigs) > 1 {
		mr.logger.Debug("渠道有多个模型匹配, 按权重选择一个",
			slog.String("trace_id", traceID),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("model", selectedConfig.Model),
			slog.Int64("selected_weight", weights[selectedIndex]),
			slog.Int("count", len(availableModelConfigs)),
		)
	}

	mr.logger.Debug("模型已路由",
		slog.String("trace_id", traceID),
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("channel_name", channel.Name),
		slog.String("model", selectedConfig.Model),
		slog.String("channel_model", selectedConfig.ChannelModel),
		slog.Any("key_groups", selectedConfig.KeyGroups),
	)
	return selectedConfig
}
