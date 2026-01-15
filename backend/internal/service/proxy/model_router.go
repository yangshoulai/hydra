package proxy

import (
	"errors"
	"log/slog"

	"github.com/yangshoulai/hydra/internal/models"
)

var (
	// ErrModelNotFound 模型不存在
	ErrModelNotFound = errors.New("model not found in channel")
	// ErrNoModelMapping 无模型映射配置
	ErrNoModelMapping = errors.New("no model mapping configuration")
)

// ModelRouter 模型路由器,负责统一模型名到上游模型名的映射
type ModelRouter struct {
	logger *slog.Logger
}

// NewModelRouter 创建模型路由器
func NewModelRouter(logger *slog.Logger) *ModelRouter {
	return &ModelRouter{
		logger: logger,
	}
}

// RouteModel 将统一模型名路由到上游模型名
// unifiedModel: 用户请求的统一模型名(如 gpt-4)
// channel: 选定的渠道
// 返回: 上游真实模型名(如 gpt-4-0613)
func (mr *ModelRouter) RouteModel(unifiedModel string, channel *models.Channel) (string, error) {
	if channel == nil {
		return "", errors.New("channel is nil")
	}

	if len(channel.ModelConfigs) == 0 {
		mr.logger.Warn("channel has no model configurations",
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
		)
		return "", ErrNoModelMapping
	}

	// 查找匹配的模型配置
	var matchedConfigs []models.ChannelModelConfig
	for _, config := range channel.ModelConfigs {
		if config.UnifiedModel == unifiedModel && config.IsActive() {
			matchedConfigs = append(matchedConfigs, config)
		}
	}

	if len(matchedConfigs) == 0 {
		mr.logger.Warn("no matching model configuration found",
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("channel_name", channel.Name),
			slog.String("unified_model", unifiedModel),
		)
		return "", ErrModelNotFound
	}

	// 如果有多个配置,选择第一个(理论上不应该出现重复配置)
	selectedConfig := matchedConfigs[0]
	if len(matchedConfigs) > 1 {
		mr.logger.Warn("multiple model configurations found, using first one",
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("unified_model", unifiedModel),
			slog.Int("count", len(matchedConfigs)),
		)
	}

	mr.logger.Debug("model routed",
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("channel_name", channel.Name),
		slog.String("unified_model", unifiedModel),
		slog.String("upstream_model", selectedConfig.UpstreamModel),
	)

	return selectedConfig.UpstreamModel, nil
}

// ReverseRoute 反向路由,将上游模型名映射回统一模型名
// 用于响应处理,将上游返回的模型名转换为统一模型名
func (mr *ModelRouter) ReverseRoute(upstreamModel string, channel *models.Channel) (string, error) {
	if channel == nil {
		return "", errors.New("channel is nil")
	}

	if len(channel.ModelConfigs) == 0 {
		return "", ErrNoModelMapping
	}

	// 查找匹配的模型配置
	for _, config := range channel.ModelConfigs {
		if config.UpstreamModel == upstreamModel && config.IsActive() {
			mr.logger.Debug("model reverse routed",
				slog.Uint64("channel_id", uint64(channel.ID)),
				slog.String("upstream_model", upstreamModel),
				slog.String("unified_model", config.UnifiedModel),
			)
			return config.UnifiedModel, nil
		}
	}

	mr.logger.Warn("no matching upstream model configuration found",
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("upstream_model", upstreamModel),
	)

	// 如果找不到映射,返回原始模型名
	return upstreamModel, nil
}

// GetSupportedModels 获取指定渠道支持的所有统一模型名
func (mr *ModelRouter) GetSupportedModels(channel *models.Channel) []string {
	if channel == nil || len(channel.ModelConfigs) == 0 {
		return []string{}
	}

	modelsMap := make(map[string]bool)
	for _, config := range channel.ModelConfigs {
		if config.IsActive() {
			modelsMap[config.UnifiedModel] = true
		}
	}

	models := make([]string, 0, len(modelsMap))
	for model := range modelsMap {
		models = append(models, model)
	}

	return models
}

// ValidateModel 验证统一模型名在指定渠道中是否存在
func (mr *ModelRouter) ValidateModel(unifiedModel string, channel *models.Channel) bool {
	if channel == nil || len(channel.ModelConfigs) == 0 {
		return false
	}

	for _, config := range channel.ModelConfigs {
		if config.UnifiedModel == unifiedModel && config.IsActive() {
			return true
		}
	}

	return false
}
