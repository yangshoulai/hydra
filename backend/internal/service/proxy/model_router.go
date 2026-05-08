package proxy

import "log/slog"

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

// RouteModel 在所有可用渠道模型配置中按模型配置权重路由。
// 渠道权重不参与请求期选择，只作为渠道模型配置创建时的初始权重来源。
// candidates: 已经过渠道、模型配置、Key 分组、熔断状态过滤后的候选渠道模型
// traceID: 请求追踪ID
// 返回: 匹配的渠道模型候选
func (mr *ModelRouter) RouteModel(candidates []*modelRouteCandidate, traceID string) *modelRouteCandidate {
	if len(candidates) == 0 {
		return nil
	}

	weights := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil {
			weights = append(weights, 0)
			continue
		}
		weights = append(weights, candidate.weight)
	}

	selectedIndex := weightedRandomIndex(weights)
	if selectedIndex < 0 || selectedIndex >= len(candidates) {
		return nil
	}
	selectedCandidate := candidates[selectedIndex]
	if selectedCandidate == nil || selectedCandidate.channel == nil {
		return nil
	}
	selectedConfig := selectedCandidate.config

	if len(candidates) > 1 {
		mr.logger.Debug("多个渠道模型匹配，按渠道模型权重选择一个",
			slog.String("trace_id", traceID),
			slog.Uint64("channel_id", uint64(selectedCandidate.channel.ID)),
			slog.String("model", selectedConfig.Model),
			slog.String("channel_model", selectedConfig.ChannelModel),
			slog.Int64("selected_weight", selectedCandidate.weight),
			slog.Int("count", len(candidates)),
		)
	}

	mr.logger.Debug("模型已路由",
		slog.String("trace_id", traceID),
		slog.Uint64("channel_id", uint64(selectedCandidate.channel.ID)),
		slog.String("channel_name", selectedCandidate.channel.Name),
		slog.String("model", selectedConfig.Model),
		slog.String("channel_model", selectedConfig.ChannelModel),
		slog.Any("key_groups", selectedConfig.KeyGroups),
	)
	return selectedCandidate
}
