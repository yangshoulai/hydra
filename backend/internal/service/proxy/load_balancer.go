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
	UpstreamModel string          // 上游模型名
	UnifiedModel  string          // 统一模型名
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
	modelRouter := NewModelRouter(logger)

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
	// 1. 选择 Channel (同时考虑模型和端点类型)
	channel, err := lb.channelSelector.SelectChannel(ctx, unifiedModel, endpointType, traceID)
	if err != nil {
		lb.logger.Warn("选择渠道失败", slog.String("trace_id", traceID), slog.String("unified_model", unifiedModel), slog.String("endpoint_type", endpointType), slog.String("error", err.Error()))
		return nil, err
	}

	// 2. 选择 Key
	key, err := lb.keySelector.SelectKey(channel, traceID)
	if err != nil {
		// 如果是没有可用的 key，返回 ErrNoAvailableChannel 让上层尝试其他 channel
		if errors.Is(err, ErrNoAvailableKey) {
			lb.logger.Debug("渠道当前没有可用密钥，尝试别的渠道", slog.String("trace_id", traceID), slog.Uint64("channel_id", uint64(channel.ID)), slog.String("channel_name", channel.Name))
			return nil, ErrNoAvailableChannel
		}
		lb.logger.Warn("选择密钥失败", slog.String("trace_id", traceID), slog.Uint64("channel_id", uint64(channel.ID)), slog.String("channel_name", channel.Name), slog.String("error", err.Error()))
		return nil, err
	}

	// 3. 路由模型
	upstreamModel, err := lb.modelRouter.RouteModel(unifiedModel, channel, endpointType, traceID)
	if err != nil {
		lb.logger.Warn("模型路由失败", slog.String("trace_id", traceID), slog.Uint64("channel_id", uint64(channel.ID)), slog.String("channel_name", channel.Name), slog.String("unified_model", unifiedModel), slog.String("error", err.Error()))
		return nil, err
	}

	result := &RouteResult{
		Channel:       channel,
		Key:           key,
		UpstreamModel: upstreamModel,
		UnifiedModel:  unifiedModel,
	}

	lb.logger.Info("请求路由成功", slog.String("trace_id", traceID), slog.Uint64("channel_id", uint64(channel.ID)), slog.String("channel_name", channel.Name), slog.Uint64("key_id", uint64(key.ID)), slog.String("unified_model", unifiedModel), slog.String("upstream_model", upstreamModel))

	return result, nil
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
	traceID string,
) (*RouteResult, error) {
	excludeMap := make(map[uint]bool)
	for _, channelID := range excludeChannels {
		excludeMap[channelID] = true
	}

	for i := 0; i <= maxRetries; i++ {
		result, err := lb.Route(ctx, unifiedModel, endpointType, traceID)
		if err != nil {
			// 检查 context 是否被取消或超时
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				lb.logger.Debug("请求上下文已取消", slog.String("trace_id", traceID), slog.String("error", err.Error()))
				return nil, err
			}

			// 如果是无可用 Channel,直接返回
			if errors.Is(err, ErrNoAvailableChannel) {
				return nil, err
			}
			// 其他错误,继续重试
			continue
		}

		// 检查是否已在排除列表中
		if excludeMap[result.Channel.ID] {
			lb.logger.Debug("渠道已经尝试过, 再试一次", slog.String("trace_id", traceID), slog.Uint64("channel_id", uint64(result.Channel.ID)), slog.Int("retry_count", i))
			continue
		}

		return result, nil
	}

	lb.logger.Error("所有重试尝试均失败", slog.String("trace_id", traceID), slog.String("unified_model", unifiedModel), slog.Int("max_retries", maxRetries))

	return nil, ErrNoAvailableRoute
}

// GetAvailableChannelCount 获取可用渠道数量
func (lb *LoadBalancer) GetAvailableChannelCount(ctx context.Context, unifiedModel string) (int, error) {
	return lb.channelSelector.GetAvailableChannelCount(ctx, unifiedModel)
}

// ValidateModel 验证模型是否存在可用路由
func (lb *LoadBalancer) ValidateModel(ctx context.Context, unifiedModel string) (bool, error) {
	count, err := lb.GetAvailableChannelCount(ctx, unifiedModel)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
