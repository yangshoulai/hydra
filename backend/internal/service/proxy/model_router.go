package proxy

import (
	"log/slog"
	"math/rand/v2"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/service/circuit"
)

// ModelRouter 模型路由器,负责统一模型名到上游模型名的映射
type ModelRouter struct {
	logger         *slog.Logger
	circuitManager *circuit.Manager
	keySelector    *KeySelector
}

// NewModelRouter 创建模型路由器
func NewModelRouter(logger *slog.Logger, circuitManager *circuit.Manager, keySelector *KeySelector) *ModelRouter {
	return &ModelRouter{
		logger:         logger,
		circuitManager: circuitManager,
		keySelector:    keySelector,
	}
}

// RouteModel 将统一模型名路由到上游模型名
// channel: 选定的渠道
// availableModelConfigs: 可用渠道模型
// selectedKey: 选择的密钥
// traceID: 请求追踪ID
// 返回: 匹配的模型配置
func (mr *ModelRouter) RouteModel(channel *models.Channel, availableModelConfigs []models.ChannelModelConfig, selectedKey models.Key, traceID string) models.ChannelModelConfig {

	// 根据密钥分组过去模型配置
	filteredConfigs := make([]models.ChannelModelConfig, len(availableModelConfigs))
	for _, config := range availableModelConfigs {
		for _, kg := range config.KeyGroups {
			if selectedKey.KeyGroup == kg {
				filteredConfigs = append(filteredConfigs, config)
			}
		}
	}

	// 如果有多个配置,随机选择一个实现负载均衡
	selectedConfig := filteredConfigs[rand.IntN(len(filteredConfigs))]
	if len(filteredConfigs) > 1 {
		mr.logger.Debug("渠道有多个模型匹配, 随机选择一个",
			slog.String("trace_id", traceID),
			slog.Uint64("channel_id", uint64(channel.ID)),
			slog.String("unified_model", selectedConfig.UnifiedModel),
			slog.String("key_group", selectedKey.KeyGroup),
			slog.Int("count", len(filteredConfigs)),
		)
	}

	mr.logger.Debug("模型已路由",
		slog.String("trace_id", traceID),
		slog.Uint64("channel_id", uint64(channel.ID)),
		slog.String("channel_name", channel.Name),
		slog.String("unified_model", selectedConfig.UnifiedModel),
		slog.String("upstream_model", selectedConfig.UpstreamModel),
		slog.String("key_group", selectedKey.KeyGroup),
	)
	return selectedConfig
}
