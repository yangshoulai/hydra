package proxy

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/endpoint"
	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
	configService "github.com/yangshoulai/hydra/internal/service/config"
	loggerService "github.com/yangshoulai/hydra/internal/service/logger"
	"github.com/yangshoulai/hydra/internal/service/metrics"
)

// 失败阶段标识，写入 proxyCtx.LastFailureStage 便于日志聚合排障。
const (
	stageRoute            = "route"
	stageStreamProbe      = "stream_probe"
	stageStreamForward    = "stream_forward"
	stageNonStreamRead    = "non_stream_read"
	stageNonStreamForward = "non_stream_forward"
	stageHandleResponse   = "handle_response"
	stageClientCancelled  = "client_cancelled"
)

// clientCancelledStatusCode nginx 惯例：客户端主动关闭连接
const clientCancelledStatusCode = 499

// ProxyService 代理服务主逻辑
//
// 请求处理分为五个顺序阶段：
//  1. prepareRequest       解析请求体、校验令牌权限与模型可用性
//  2. routeAndProxy        负载均衡 + 重试循环的外壳
//  3. attemptOnce          单次尝试：构建请求 → 发送 → 响应分发
//  4. handleStreamUpstream / handleNonStreamUpstream   嗅探、转发、成功统计
//  5. logFinalRequestResult    最终日志汇总（defer 触发）
//
// 设计原则：
//   - 失败归因集中在 tryScheduleRetry + recordFailure，降低控制流复杂度
//   - 仅持久化 token 使用量（异步），其余明细走日志
//   - 客户端主动断开与上游失败严格区分
//
// 代码按职责分布：
//   - proxy_service.go   struct / 构造 / 主入口 / 路由编排 / 配置热更新
//   - proxy_attempt.go   prepare / attempt / stream & non-stream handler / 重试 / 指标
//   - proxy_logging.go   trace 日志 / 请求汇总 / 请求日志持久化
//   - proxy_helpers.go   free-standing 工具函数
type ProxyService struct {
	logger            *slog.Logger
	loadBalancer      *LoadBalancer
	requestBuilder    *RequestBuilder
	httpClient        *HTTPClient
	responseSniffer   *ResponseSniffer
	responseForwarder *ResponseForwarder
	failureClassifier *FailureClassifier
	retryCoordinator  *RetryCoordinator
	circuitManager    *circuit.CircuitManager
	channelKeyRepo    *repository.ChannelKeyRepository
	modelConfigRepo   *repository.ChannelModelConfigRepository
	accessTokenRepo   *repository.AccessTokenRepository
	settingService    *configService.SettingService
	runtimeMetrics    *metrics.RuntimeMetrics

	tokenUsageRecorder *TokenUsageRecorder
	requestLogRecorder *RequestLogRecorder

	snifferConfigMu         sync.RWMutex
	streamSnifferEnabled    bool
	nonStreamSnifferEnabled bool
	streamSniffPacketCount  int
	debugModeEnabled        atomic.Bool
	streamKeepaliveNanos    atomic.Int64
}

// ProxyServiceConfig 代理服务配置
type ProxyServiceConfig struct {
	MaxRetries              int
	RetryDelay              time.Duration
	RequestTimeout          time.Duration
	StreamKeepaliveInterval time.Duration
	NetworkProxy            string
	LoadBalanceStrategy     string
}

// =============================================================================
// 生命周期
// =============================================================================

// NewProxyService 创建代理服务
func NewProxyService(
	logger *slog.Logger,
	channelRepo *repository.ChannelRepository,
	channelKeyRepo *repository.ChannelKeyRepository,
	modelConfigRepo *repository.ChannelModelConfigRepository,
	accessTokenRepo *repository.AccessTokenRepository,
	circuitManager *circuit.CircuitManager,
	config *ProxyServiceConfig,
	settingService *configService.SettingService,
	runtimeMetrics *metrics.RuntimeMetrics,
	requestLogRecorder *RequestLogRecorder,
) *ProxyService {
	httpClientConfig := DefaultHTTPClientConfig()
	httpClientConfig.RequestTimeout = config.RequestTimeout
	httpClientConfig.UpstreamProxyURL = config.NetworkProxy

	retryDelay := 500 * time.Millisecond
	maxRetries := 3
	if config.RetryDelay > 0 {
		retryDelay = config.RetryDelay
	}
	if config.MaxRetries > 0 {
		maxRetries = config.MaxRetries
	}

	responseSniffer := NewResponseSniffer(logger)
	snifferCfg := settingService.GetSnifferConfig(context.Background())
	responseSniffer.UpdatePlainTextErrorKeywords(snifferCfg.Keywords)

	svc := &ProxyService{
		logger:                  logger,
		loadBalancer:            NewLoadBalancer(logger, channelRepo, circuitManager, config.LoadBalanceStrategy),
		requestBuilder:          NewRequestBuilder(),
		httpClient:              NewHTTPClient(httpClientConfig, logger),
		responseSniffer:         responseSniffer,
		responseForwarder:       NewResponseForwarder(logger),
		failureClassifier:       NewFailureClassifier(),
		retryCoordinator:        NewRetryCoordinator(logger, maxRetries, retryDelay, nil),
		circuitManager:          circuitManager,
		channelKeyRepo:          channelKeyRepo,
		modelConfigRepo:         modelConfigRepo,
		accessTokenRepo:         accessTokenRepo,
		settingService:          settingService,
		runtimeMetrics:          runtimeMetrics,
		tokenUsageRecorder:      NewTokenUsageRecorder(logger, modelConfigRepo, channelKeyRepo, accessTokenRepo, 1024, 4),
		requestLogRecorder:      requestLogRecorder,
		streamSnifferEnabled:    snifferCfg.StreamEnabled,
		nonStreamSnifferEnabled: snifferCfg.NonStreamEnabled,
		streamSniffPacketCount:  normalizeStreamSniffPacketCount(snifferCfg.StreamPacketCount),
	}
	svc.debugModeEnabled.Store(settingService.GetBool(context.Background(), models.SettingLogDebugEnabled, false))
	svc.updateStreamKeepaliveInterval(config.StreamKeepaliveInterval)
	svc.retryCoordinator.SetDebugFn(svc.isDebugModeEnabled)
	return svc
}

// Close 释放异步资源。调用方（如 runtime）应在服务退出前调用。
func (ps *ProxyService) Close() {
	if ps.tokenUsageRecorder != nil {
		ps.tokenUsageRecorder.Close()
	}
	if ps.requestLogRecorder != nil {
		ps.requestLogRecorder.Close()
	}
	if ps.httpClient != nil {
		ps.httpClient.Close()
	}
}

// =============================================================================
// 对外 API
// =============================================================================

// ProxyRequest 通用代理请求入口
func (ps *ProxyService) ProxyRequest(c *gin.Context, ep endpoint.Endpoint) (retErr error) {
	traceID := GetTraceIDFromContext(c)
	proxyCtx := NewProxyContext(traceID, "", false, ep, nil)

	defer func() {
		ps.logFinalRequestResult(c, proxyCtx, retErr)
	}()

	ps.logTraceInfo(traceID, "接收代理请求",
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.String("endpoint_type", ep.GetType()),
		slog.String("client_ip", c.ClientIP()),
	)

	if retErr = ps.prepareRequest(c, ep, proxyCtx); retErr != nil {
		return retErr
	}

	ps.logTraceInfo(traceID, "模型校验通过，开始路由",
		slog.String("endpoint_type", ep.GetType()),
		slog.String("model", proxyCtx.Model),
		slog.Bool("request_stream", proxyCtx.IsStreamRequest),
	)

	retErr = ps.routeAndProxy(c, proxyCtx)
	return retErr
}

// UpdateSnifferKeywords 更新嗅探器的明文错误关键词
func (ps *ProxyService) UpdateSnifferKeywords(keywords []string) {
	ps.responseSniffer.UpdatePlainTextErrorKeywords(keywords)
	ps.logger.Info("嗅探器关键词已更新", "count", len(keywords))
}

// OnConfigChanged 实现 ConfigListener 接口，响应配置变更
func (ps *ProxyService) OnConfigChanged(ctx context.Context, category string) {
	switch category {
	case "logging":
		ps.reloadLoggingConfig(ctx)
	case "proxy":
		ps.reloadProxyConfig(ctx)
	case "sniffer":
		ps.reloadSnifferConfig(ctx)
	}
}

// =============================================================================
// 阶段 2：路由与重试循环
// =============================================================================

// routeAndProxy 每轮：路由 → 单次尝试；失败可重试则继续。
func (ps *ProxyService) routeAndProxy(c *gin.Context, proxyCtx *ProxyContext) error {
	for {
		routeResult, routeErr := ps.loadBalancer.Route(c.Request.Context(), proxyCtx)
		if routeErr != nil {
			return ps.handleRouteFailure(c, proxyCtx, routeErr)
		}

		shouldRetry, err := ps.attemptOnce(c, proxyCtx, routeResult)
		if shouldRetry {
			continue
		}
		return err
	}
}

// handleRouteFailure 路由阶段失败（无可用渠道/密钥等）
func (ps *ProxyService) handleRouteFailure(c *gin.Context, proxyCtx *ProxyContext, routeErr error) error {
	ps.logTraceInfo(proxyCtx.TraceID, "路由失败",
		slog.String("model", proxyCtx.Model),
		slog.String("error", routeErr.Error()),
	)
	proxyCtx.LastError = routeErr
	proxyCtx.LastFailureType = FailureTypeNone
	proxyCtx.LastFailureScope = FailureScopeNone
	proxyCtx.LastFailureStage = stageRoute

	if isClientCancelled(c, routeErr) {
		return ps.markClientCancelled(c, proxyCtx, routeErr)
	}

	ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, "model unavailable: "+proxyCtx.Model, proxyCtx.TraceID)
	ps.recordRequestMetrics(false, proxyCtx.Model, nil, 0, 0)
	return routeErr
}

// =============================================================================
// 配置热更新
// =============================================================================

func (ps *ProxyService) reloadLoggingConfig(ctx context.Context) {
	ps.debugModeEnabled.Store(ps.settingService.GetBool(ctx, models.SettingLogDebugEnabled, false))
	effectiveLogLevel := ps.settingService.GetEffectiveLogLevel(ctx)
	loggerService.SetLogLevel(effectiveLogLevel)
	addSource := ps.settingService.GetBool(ctx, models.SettingLogAddSource, false)
	loggerService.SetAddSource(addSource)
	ps.logger.Info("代理日志级别已热更新",
		slog.String("effective_log_level", effectiveLogLevel),
		slog.Bool("debug_mode", ps.debugModeEnabled.Load()),
		slog.Bool("add_source", addSource),
	)
}

func (ps *ProxyService) reloadProxyConfig(ctx context.Context) {
	requestTimeout, keepaliveInterval, networkProxyURL, maxRetry, loadBalanceStrategy := ps.settingService.GetProxyConfig(ctx)
	maxBodyBytes := ps.settingService.GetProxyMaxBodyBytes(ctx)
	rateLimitConfig := ps.settingService.GetProxyRateLimitConfig(ctx)
	retryDelay := 500 * time.Millisecond

	ps.httpClient.UpdateRequestTimeout(requestTimeout)
	ps.httpClient.UpdateUpstreamProxyURL(networkProxyURL)
	ps.retryCoordinator.UpdateConfig(maxRetry, retryDelay)
	ps.updateStreamKeepaliveInterval(keepaliveInterval)
	ps.loadBalancer.UpdateChannelSelectionStrategy(loadBalanceStrategy)

	ps.logger.Info("代理服务配置已更新",
		slog.Duration("request_timeout", requestTimeout),
		slog.Duration("stream_keepalive_interval", keepaliveInterval),
		slog.String("network_proxy_url", networkProxyURL),
		slog.Int("max_retry", maxRetry),
		slog.String("load_balance_strategy", ps.loadBalancer.CurrentChannelSelectionStrategyName()),
		slog.Int64("max_body_bytes", maxBodyBytes),
		slog.Bool("rate_limit_enabled", rateLimitConfig.Enabled),
		slog.Int("rate_limit_global_rps", rateLimitConfig.GlobalRPS),
		slog.Int("rate_limit_token_rps", rateLimitConfig.TokenRPS),
		slog.Duration("retry_delay", retryDelay),
	)
}

func (ps *ProxyService) reloadSnifferConfig(ctx context.Context) {
	cfg := ps.settingService.GetSnifferConfig(ctx)
	ps.updateSnifferConfig(cfg)
	ps.responseSniffer.UpdatePlainTextErrorKeywords(cfg.Keywords)
	ps.logger.Info("响应嗅探配置已更新",
		slog.Bool("stream_sniffer_enabled", cfg.StreamEnabled),
		slog.Bool("non_stream_sniffer_enabled", cfg.NonStreamEnabled),
		slog.Int("stream_packet_count", cfg.StreamPacketCount),
		slog.Int("keywords_count", len(cfg.Keywords)),
	)
}

func (ps *ProxyService) updateSnifferConfig(cfg configService.SnifferConfig) {
	ps.snifferConfigMu.Lock()
	ps.streamSnifferEnabled = cfg.StreamEnabled
	ps.nonStreamSnifferEnabled = cfg.NonStreamEnabled
	ps.streamSniffPacketCount = normalizeStreamSniffPacketCount(cfg.StreamPacketCount)
	ps.snifferConfigMu.Unlock()
}

func (ps *ProxyService) getStreamSnifferConfig() (enabled bool, streamPacketCount int) {
	ps.snifferConfigMu.RLock()
	defer ps.snifferConfigMu.RUnlock()
	return ps.streamSnifferEnabled, ps.streamSniffPacketCount
}

func (ps *ProxyService) isNonStreamSnifferEnabled() bool {
	ps.snifferConfigMu.RLock()
	defer ps.snifferConfigMu.RUnlock()
	return ps.nonStreamSnifferEnabled
}

func (ps *ProxyService) isDebugModeEnabled() bool {
	return ps.debugModeEnabled.Load()
}

func (ps *ProxyService) updateStreamKeepaliveInterval(interval time.Duration) {
	ps.streamKeepaliveNanos.Store(int64(normalizeStreamKeepaliveInterval(interval)))
}

func (ps *ProxyService) getStreamKeepaliveInterval() time.Duration {
	return time.Duration(ps.streamKeepaliveNanos.Load())
}

func (ps *ProxyService) GetHTTPClient() *HTTPClient {
	return ps.httpClient
}
