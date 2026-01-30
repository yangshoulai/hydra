package proxy

import (
	"context"
	"errors"
	"log/slog"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
)

var (
	// ErrNoAvailableRoute 无可用路由
	ErrNoAvailableRoute = errors.New("no available route for request")
)

// RouteResult 路由结果
type RouteResult struct {
	Channel       *models.Channel // 选定的渠道
	Key           *models.Key     // 选定的 Key
	ModelConfigID uint            // 选中的模型配置ID
	UpstreamModel string          // 上游模型名
	UnifiedModel  string          // 统一模型名
	KeyGroups     []string        // 模型配置对应的密钥分组
}

// LoadBalancer 负载均衡器,协调多渠道流量分配
type LoadBalancer struct {
	logger          *slog.Logger
	channelSelector *ChannelSelector
	keySelector     *KeySelector
	modelRouter     *ModelRouter
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(
	logger *slog.Logger,
	channelRepo *repository.ChannelRepository,
	circuitManager *circuit.Manager,
) *LoadBalancer {
	keySelector := NewKeySelector(logger, circuitManager)
	channelSelector := NewChannelSelector(logger, channelRepo, keySelector, circuitManager)
	modelRouter := NewModelRouter(logger, circuitManager, keySelector)

	return &LoadBalancer{
		logger:          logger,
		channelSelector: channelSelector,
		keySelector:     keySelector,
		modelRouter:     modelRouter,
	}
}

// Route 为请求路由到合适的 Channel 和 Key
// unifiedModel: 用户请求的统一模型名
// endpointType: 端点类型(如 openai, openai-response, anthropic)
// traceID: 请求追踪ID
// 返回: 路由结果(包含 Channel, Key, 上游模型名)
func (lb *LoadBalancer) Route(ctx context.Context, unifiedModel string, endpointType string, traceID string) (*RouteResult, error) {
	return lb.routeWithExclusions(ctx, unifiedModel, endpointType, nil, nil, nil, traceID)
}

// RouteWithRetry 为请求路由,支持重试(当某个渠道失败后尝试其他渠道)
// unifiedModel: 用户请求的统一模型名
// endpointType: 端点类型(如 openai, openai-response, anthropic)
// maxRetries: 最大重试次数
// excludeChannels: 排除的渠道ID列表(已经尝试过失败的)
// traceID: 请求追踪ID
func (lb *LoadBalancer) RouteWithRetry(
	ctx context.Context,
	unifiedModel string,
	endpointType string,
	maxRetries int,
	excludeChannels []uint,
	excludeModelConfigs []uint,
	excludeKeys []uint,
	traceID string,
) (*RouteResult, error) {
	excludeMap := make(map[uint]bool)
	for _, channelID := range excludeChannels {
		excludeMap[channelID] = true
	}
	excludeModelMap := make(map[uint]bool)
	for _, modelID := range excludeModelConfigs {
		excludeModelMap[modelID] = true
	}
	excludeKeyMap := make(map[uint]bool)
	for _, keyID := range excludeKeys {
		excludeKeyMap[keyID] = true
	}

	for i := 0; i <= maxRetries; i++ {
		result, err := lb.routeWithExclusions(ctx, unifiedModel, endpointType, excludeMap, excludeModelMap, excludeKeyMap, traceID)
		if err != nil {
			// 检查 context 是否被取消或超时
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				lb.logger.Debug("请求已取消", slog.String("trace_id", traceID), slog.String("error", err.Error()))
				return nil, err
			}

			// 如果是无可用 Channel,直接返回
			if errors.Is(err, ErrNoAvailableChannel) {
				if len(excludeMap) > 0 {
					fallbackResult, fallbackErr := lb.routeWithExclusions(ctx, unifiedModel, endpointType, nil, excludeModelMap, excludeKeyMap, traceID)
					if fallbackErr == nil {
						return fallbackResult, nil
					}
					return nil, fallbackErr
				}
				return nil, err
			}
			// 其他错误,继续重试
			continue
		}
		return result, nil
	}

	lb.logger.Debug("所有重试尝试均失败", slog.String("trace_id", traceID), slog.String("unified_model", unifiedModel), slog.Int("max_retries", maxRetries))

	return nil, ErrNoAvailableRoute
}

func (lb *LoadBalancer) routeWithExclusions(
	ctx context.Context,
	unifiedModel string,
	endpointType string,
	excludeChannels map[uint]bool,
	excludeModelConfigs map[uint]bool,
	excludeKeys map[uint]bool,
	traceID string,
) (*RouteResult, error) {
	// 1. 选择 Channel (同时考虑模型和端点类型)
	channel, err := lb.channelSelector.SelectChannel(ctx, unifiedModel, endpointType, traceID, excludeChannels)
	if err != nil {
		return nil, err
	}

	availableConfigs := channel.ModelConfigs
	availableKeys := channel.Keys
	if len(availableConfigs) == 0 || len(availableKeys) == 0 {
		return nil, ErrNoAvailableRoute
	}

	preferredConfigs := filterModelConfigs(availableConfigs, excludeModelConfigs)
	preferredKeyGroups := collectKeyGroups(preferredConfigs)
	nonExcludedKeys := filterKeys(availableKeys, excludeKeys)
	preferredKeys := filterKeysByGroup(nonExcludedKeys, preferredKeyGroups)

	keysForSelection := preferredKeys
	if len(keysForSelection) == 0 {
		if len(nonExcludedKeys) > 0 {
			keysForSelection = nonExcludedKeys
		} else {
			keysForSelection = availableKeys
		}
	}
	if len(keysForSelection) == 0 {
		return nil, ErrNoAvailableRoute
	}

	// 2. 选择密钥（优先未排除的 Key）
	selectedKey := lb.keySelector.SelectKey(channel, keysForSelection)

	// 3. 路由模型（优先未排除的模型配置）
	configsForKey := filterConfigsByGroup(preferredConfigs, selectedKey.KeyGroup)
	if len(configsForKey) == 0 {
		configsForKey = filterConfigsByGroup(availableConfigs, selectedKey.KeyGroup)
	}
	if len(configsForKey) == 0 {
		return nil, ErrNoAvailableRoute
	}

	selectedConfig := lb.modelRouter.RouteModel(channel, configsForKey, selectedKey, traceID)

	result := &RouteResult{
		Channel:       channel,
		Key:           &selectedKey,
		ModelConfigID: selectedConfig.ID,
		UpstreamModel: selectedConfig.UpstreamModel,
		UnifiedModel:  unifiedModel,
		KeyGroups:     selectedConfig.KeyGroups,
	}
	return result, nil
}

func filterModelConfigs(configs []models.ChannelModelConfig, exclude map[uint]bool) []models.ChannelModelConfig {
	if exclude == nil || len(exclude) == 0 {
		return configs
	}
	result := make([]models.ChannelModelConfig, 0, len(configs))
	for _, config := range configs {
		if !exclude[config.ID] {
			result = append(result, config)
		}
	}
	return result
}

func filterKeys(keys []models.Key, exclude map[uint]bool) []models.Key {
	if exclude == nil || len(exclude) == 0 {
		return keys
	}
	result := make([]models.Key, 0, len(keys))
	for _, key := range keys {
		if !exclude[key.ID] {
			result = append(result, key)
		}
	}
	return result
}

func collectKeyGroups(configs []models.ChannelModelConfig) map[string]struct{} {
	groups := make(map[string]struct{})
	for _, config := range configs {
		for _, group := range config.KeyGroups {
			groups[group] = struct{}{}
		}
	}
	return groups
}

func filterKeysByGroup(keys []models.Key, groups map[string]struct{}) []models.Key {
	if len(groups) == 0 {
		return nil
	}
	result := make([]models.Key, 0, len(keys))
	for _, key := range keys {
		if _, ok := groups[key.KeyGroup]; ok {
			result = append(result, key)
		}
	}
	return result
}

func filterConfigsByGroup(configs []models.ChannelModelConfig, keyGroup string) []models.ChannelModelConfig {
	result := make([]models.ChannelModelConfig, 0, len(configs))
	for _, config := range configs {
		for _, group := range config.KeyGroups {
			if group == keyGroup {
				result = append(result, config)
				break
			}
		}
	}
	return result
}
