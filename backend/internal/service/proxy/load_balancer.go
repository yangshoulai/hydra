package proxy

import (
	"context"
	"log/slog"
	"sync"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
)

// RouteResult 路由结果
type RouteResult struct {
	Channel       *models.Channel    // 选定的渠道
	Key           *models.ChannelKey // 选定的 Key
	ModelConfigID uint               // 选中的模型配置ID
	ChannelModel  string             // 渠道模型名
	Model         string             // 统一模型名
	ModelWeight   int                // 模型配置权重
	KeyGroups     []string           // 模型配置对应的密钥分组
}

type channelRouteCandidate struct {
	channel *models.Channel
	configs []models.ChannelModelConfig
	keys    []models.ChannelKey
}

// LoadBalancer 负载均衡器,协调多渠道流量分配
type LoadBalancer struct {
	channelSelector *ChannelSelector
	keySelector     *KeySelector
	modelRouter     *ModelRouter
	strategyMu      sync.RWMutex
	channelStrategy channelSelectionStrategy
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(
	logger *slog.Logger,
	channelRepo *repository.ChannelRepository,
	circuitManager *circuit.CircuitManager,
	strategyName string,
) *LoadBalancer {
	keySelector := NewKeySelector()
	channelSelector := NewChannelSelector(channelRepo, circuitManager)
	modelRouter := NewModelRouter(logger)

	loadBalancer := &LoadBalancer{
		channelSelector: channelSelector,
		keySelector:     keySelector,
		modelRouter:     modelRouter,
	}
	loadBalancer.UpdateChannelSelectionStrategy(strategyName)
	return loadBalancer
}

// Route 为请求路由到合适的 Channel 和 Key
// 通过 ProxyContext 读取模型、端点类型、TraceID 以及已失败的排除集合。
func (lb *LoadBalancer) Route(ctx context.Context, proxyCtx *ProxyContext) (*RouteResult, error) {
	excludeMap := make(map[uint]bool)
	for _, channelID := range proxyCtx.FailedChannelIDs {
		excludeMap[channelID] = true
	}
	excludeModelMap := make(map[uint]bool)
	for _, modelID := range proxyCtx.FailedModelIDs {
		excludeModelMap[modelID] = true
	}
	excludeKeyMap := make(map[uint]bool)
	for _, keyID := range proxyCtx.FailedKeyIDs {
		excludeKeyMap[keyID] = true
	}
	lastChannelID := uint(0)
	if proxyCtx.LastRoute != nil {
		lastChannelID = proxyCtx.LastRoute.ChannelID
	}

	result, err := lb.routeWithExclusions(
		ctx,
		proxyCtx.Model,
		proxyCtx.Endpoint.GetType(),
		proxyCtx.IsStreamRequest,
		excludeMap,
		excludeModelMap,
		excludeKeyMap,
		proxyCtx.TraceID,
		lastChannelID,
	)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (lb *LoadBalancer) routeWithExclusions(
	ctx context.Context,
	model string,
	endpointType string,
	isStream bool,
	excludeChannels map[uint]bool,
	excludeModelConfigs map[uint]bool,
	excludeKeys map[uint]bool,
	traceID string,
	lastChannelID uint,
) (*RouteResult, error) {
	channels, err := lb.channelSelector.SelectChannels(ctx, model, endpointType, isStream, excludeChannels)
	if err != nil {
		return nil, err
	}

	candidates := make([]*channelRouteCandidate, 0, len(channels))
	for i := range channels {
		routeCandidate, ok := buildChannelRouteCandidate(&channels[i], excludeModelConfigs, excludeKeys)
		if ok {
			candidates = append(candidates, routeCandidate)
		}
	}

	if len(candidates) == 0 {
		return nil, ErrNoAvailableRoute
	}

	selectedCandidate := lb.selectChannelCandidate(candidates, channelSelectionContext{
		Model:         model,
		EndpointType:  endpointType,
		IsStream:      isStream,
		LastChannelID: lastChannelID,
	})
	if selectedCandidate == nil {
		return nil, ErrNoAvailableRoute
	}

	routeResult, ok := lb.selectRouteFromChannelCandidate(selectedCandidate, traceID)
	if !ok {
		return nil, ErrNoAvailableRoute
	}
	return routeResult, nil
}

func channelWeight(channel *models.Channel) int64 {
	if channel == nil {
		return defaultRouteWeight
	}
	return normalizedWeight(channel.Weight)
}

func modelConfigWeight(config models.ChannelModelConfig, channel *models.Channel) int {
	return int(modelConfigWeightWithFallback(config, channelWeight(channel)))
}

func modelConfigWeightWithFallback(config models.ChannelModelConfig, fallbackWeight int64) int64 {
	if config.Weight > 0 {
		return int64(config.Weight)
	}
	if fallbackWeight > 0 {
		return fallbackWeight
	}
	return defaultRouteWeight
}

func buildChannelRouteCandidate(
	channel *models.Channel,
	excludeModelConfigs map[uint]bool,
	excludeKeys map[uint]bool,
) (*channelRouteCandidate, bool) {
	candidateConfigs := filterModelConfigs(channel.ModelConfigs, excludeModelConfigs)
	if len(candidateConfigs) == 0 {
		return nil, false
	}

	candidateKeys := filterKeys(channel.ChannelKeys, excludeKeys)
	if len(candidateKeys) == 0 {
		return nil, false
	}

	candidateConfigs = filterConfigsWithAvailableKeys(candidateConfigs, candidateKeys)
	if len(candidateConfigs) == 0 {
		return nil, false
	}

	return &channelRouteCandidate{
		channel: channel,
		configs: candidateConfigs,
		keys:    candidateKeys,
	}, true
}

func (lb *LoadBalancer) selectRouteFromChannelCandidate(
	candidate *channelRouteCandidate,
	traceID string,
) (*RouteResult, bool) {
	selectedConfig := lb.modelRouter.RouteModel(candidate.channel, candidate.configs, traceID)
	selectedKeys := filterKeysByConfig(candidate.keys, selectedConfig)
	if len(selectedKeys) == 0 {
		return nil, false
	}

	selectedKey := lb.keySelector.SelectKey(candidate.channel, selectedKeys)

	result := &RouteResult{
		Channel:       candidate.channel,
		Key:           &selectedKey,
		ModelConfigID: selectedConfig.ID,
		ChannelModel:  selectedConfig.ChannelModel,
		Model:         selectedConfig.Model,
		ModelWeight:   modelConfigWeight(selectedConfig, candidate.channel),
		KeyGroups:     selectedConfig.KeyGroups,
	}
	return result, true
}

func (lb *LoadBalancer) UpdateChannelSelectionStrategy(strategyName string) {
	lb.strategyMu.Lock()
	lb.channelStrategy = newChannelSelectionStrategy(strategyName)
	lb.strategyMu.Unlock()
}

func (lb *LoadBalancer) CurrentChannelSelectionStrategyName() string {
	lb.strategyMu.RLock()
	defer lb.strategyMu.RUnlock()
	if lb.channelStrategy == nil {
		return models.ProxyLoadBalanceStrategyWeightedRandom
	}
	return lb.channelStrategy.Name()
}

func (lb *LoadBalancer) selectChannelCandidate(
	candidates []*channelRouteCandidate,
	ctx channelSelectionContext,
) *channelRouteCandidate {
	lb.strategyMu.RLock()
	strategy := lb.channelStrategy
	lb.strategyMu.RUnlock()
	if strategy == nil {
		strategy = newChannelSelectionStrategy(models.ProxyLoadBalanceStrategyWeightedRandom)
	}
	return strategy.Select(candidates, ctx)
}

func filterModelConfigs(configs []models.ChannelModelConfig, exclude map[uint]bool) []models.ChannelModelConfig {
	if len(exclude) == 0 {
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

func filterKeys(keys []models.ChannelKey, exclude map[uint]bool) []models.ChannelKey {
	if len(exclude) == 0 {
		return keys
	}
	result := make([]models.ChannelKey, 0, len(keys))
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

func collectKeyGroupsFromConfig(config models.ChannelModelConfig) map[string]struct{} {
	groups := make(map[string]struct{}, len(config.KeyGroups))
	for _, group := range config.KeyGroups {
		groups[group] = struct{}{}
	}
	return groups
}

func collectKeyGroupsFromKeys(keys []models.ChannelKey) map[string]struct{} {
	groups := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		groups[key.ChannelKeyGroup] = struct{}{}
	}
	return groups
}

func filterConfigsWithAvailableKeys(configs []models.ChannelModelConfig, keys []models.ChannelKey) []models.ChannelModelConfig {
	availableGroups := collectKeyGroupsFromKeys(keys)
	result := make([]models.ChannelModelConfig, 0, len(configs))
	for _, config := range configs {
		for _, group := range config.KeyGroups {
			if _, ok := availableGroups[group]; ok {
				result = append(result, config)
				break
			}
		}
	}
	return result
}

func filterKeysByGroup(keys []models.ChannelKey, groups map[string]struct{}) []models.ChannelKey {
	if len(groups) == 0 {
		return nil
	}
	result := make([]models.ChannelKey, 0, len(keys))
	for _, key := range keys {
		if _, ok := groups[key.ChannelKeyGroup]; ok {
			result = append(result, key)
		}
	}
	return result
}

func filterKeysByConfig(keys []models.ChannelKey, config models.ChannelModelConfig) []models.ChannelKey {
	return filterKeysByGroup(keys, collectKeyGroupsFromConfig(config))
}
