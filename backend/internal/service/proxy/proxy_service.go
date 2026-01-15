package proxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
	"github.com/yangshoulai/hydra/internal/service/logger"
	"github.com/yangshoulai/hydra/internal/service/sniffer"
)

// ProxyService 代理服务主逻辑
type ProxyService struct {
	logger            *slog.Logger
	loadBalancer      *LoadBalancer
	requestBuilder    *RequestBuilder
	httpClient        *HTTPClient
	responseSniffer   *sniffer.ResponseSniffer
	sseForwarder      *SSEForwarder
	responseForwarder *ResponseForwarder
	failureClassifier *FailureClassifier
	retryCoordinator  *RetryCoordinator
	circuitManager    *circuit.Manager
	modelRouter       *ModelRouter
	auditLogger       *logger.AuditLogger // 审计日志记录器
}

// ProxyServiceConfig 代理服务配置
type ProxyServiceConfig struct {
	MaxRetries     int
	RetryDelay     time.Duration
	RequestTimeout time.Duration
}

// NewProxyService 创建代理服务
func NewProxyService(
	logger *slog.Logger,
	channelRepo *repository.ChannelRepository,
	circuitManager *circuit.Manager,
	auditLogger *logger.AuditLogger,
	config *ProxyServiceConfig,
) *ProxyService {
	loadBalancer := NewLoadBalancer(logger, channelRepo, circuitManager)
	httpClientConfig := DefaultHTTPClientConfig()
	if config != nil && config.RequestTimeout > 0 {
		httpClientConfig.RequestTimeout = config.RequestTimeout
	}

	retryDelay := 500 * time.Millisecond
	maxRetries := 3
	if config != nil {
		if config.RetryDelay > 0 {
			retryDelay = config.RetryDelay
		}
		if config.MaxRetries > 0 {
			maxRetries = config.MaxRetries
		}
	}

	return &ProxyService{
		logger:            logger,
		loadBalancer:      loadBalancer,
		requestBuilder:    NewRequestBuilder(),
		httpClient:        NewHTTPClient(httpClientConfig, logger),
		responseSniffer:   sniffer.NewResponseSniffer(logger),
		sseForwarder:      NewSSEForwarder(logger),
		responseForwarder: NewResponseForwarder(logger),
		failureClassifier: NewFailureClassifier(),
		retryCoordinator:  NewRetryCoordinator(logger, maxRetries, retryDelay),
		circuitManager:    circuitManager,
		modelRouter:       NewModelRouter(logger),
		auditLogger:       auditLogger,
	}
}

// ProxyChatCompletions 代理 Chat Completions 请求
func (ps *ProxyService) ProxyChatCompletions(c *gin.Context) error {
	ctx := c.Request.Context()
	startTime := time.Now()
	traceID := GetTraceIDFromContext(c)

	// 1. 读取请求 Body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadRequest, "Failed to read request body")
		ps.logRequestError(ctx, traceID, c, "read_request_body", err, startTime, nil, "")
		return err
	}
	// 重置 Body 使其可以再次读取
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 2. 解析请求获取模型名
	unifiedModel, err := ps.requestBuilder.GetModelFromRequest(bodyBytes)
	if err != nil {
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		ps.logRequestError(ctx, traceID, c, "parse_model", err, startTime, nil, "")
		return err
	}

	isStream := ps.requestBuilder.IsStreamRequest(bodyBytes)

	ps.logger.Info("processing chat completion request",
		slog.String("model", unifiedModel),
		slog.Bool("stream", isStream),
	)

	// 2. 创建重试上下文
	retryCtx := NewRetryContext()

	// 3. 重试循环
	for {
		// 路由到 Channel 和 Key
		routeResult, err := ps.loadBalancer.RouteWithRetry(
			ctx,
			unifiedModel,
			ps.retryCoordinator.maxRetries-retryCtx.AttemptCount,
			retryCtx.FailedChannelIDs,
		)

		if err != nil {
			ps.logger.Error("failed to route request",
				slog.String("model", unifiedModel),
				slog.String("error", err.Error()),
			)
			ps.responseForwarder.ForwardErrorResponse(c, http.StatusServiceUnavailable,
				"No available channels for model: "+unifiedModel)
			ps.logRequestError(ctx, traceID, c, "route_failed", err, startTime, nil, unifiedModel)
			return err
		}

		// 构建上游请求
		upstreamReq, _, err := ps.requestBuilder.BuildProxyRequest(c, routeResult, "/v1/chat/completions")
		if err != nil {
			ps.logger.Error("failed to build proxy request", slog.String("error", err.Error()))
			continue
		}

		// 发送请求
		upstreamResp, err := ps.httpClient.Do(upstreamReq)

		// 分类故障
		failureType := ps.failureClassifier.ClassifyResponseError(upstreamResp, err)

		if err != nil {
			// 网络错误
			ps.recordFailure(routeResult, failureType)
			ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, err, failureType)

			// 记录这次失败的尝试
			ps.logRequestError(ctx, traceID, c, "upstream_request_failed", err, startTime, routeResult, unifiedModel)

			if !ps.retryCoordinator.ShouldRetry(retryCtx) {
				ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway,
					"Upstream request failed: "+err.Error())
				return err
			}

			// 等待后重试
			ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
			continue
		}

		// 检查假 200 响应
		if upstreamResp.StatusCode == http.StatusOK {
			sniffResult, err := ps.responseSniffer.SniffResponse(upstreamResp)
			if err != nil {
				ps.logger.Error("failed to sniff response",
					slog.String("error", err.Error()),
				)
			} else if sniffResult.IsFake200 {
				ps.logger.Warn("fake 200 response detected",
					slog.String("rule", sniffResult.MatchedRule),
				)

				// 视为软故障
				ps.recordFailure(routeResult, FailureTypeSoft)
				ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID,
					errors.New("fake 200 response"), FailureTypeSoft)

				// 记录这次失败的尝试
				ps.logRequestError(ctx, traceID, c, "fake_200_response", errors.New("fake 200 response"), startTime, routeResult, unifiedModel)

				if !ps.retryCoordinator.ShouldRetry(retryCtx) {
					ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway,
						"All upstream attempts failed")
					return errors.New("fake 200 response")
				}

				ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
				continue
			}
		}

		// 检查 HTTP 错误
		if upstreamResp.StatusCode >= 400 {
			ps.recordFailure(routeResult, failureType)

			if ps.failureClassifier.ShouldRetry(failureType) {
				ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID,
					errors.New("upstream error"), failureType)

				// 记录这次失败的尝试
				ps.logRequestError(ctx, traceID, c, "upstream_http_error", errors.New("upstream error"), startTime, routeResult, unifiedModel)

				if ps.retryCoordinator.ShouldRetry(retryCtx) {
					ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
					continue
				}
			}

			// 转发错误响应
			_, err = ps.responseForwarder.ForwardResponse(c, upstreamResp)
			return err
		}

		// 成功响应
		ps.circuitManager.RecordKeySuccess(routeResult.Key.ID, routeResult.Channel.ID)

		// 转发响应
		if isStream {
			err = ps.sseForwarder.ForwardStream(c, upstreamResp)
		} else {
			_, err = ps.responseForwarder.ForwardJSONResponse(c, upstreamResp,
				routeResult.UpstreamModel, routeResult.UnifiedModel)
		}

		// 记录成功的审计日志
		if err == nil {
			ps.logRequestSuccess(ctx, traceID, c, routeResult, upstreamResp.StatusCode, startTime, isStream)
		} else {
			ps.logRequestError(ctx, traceID, c, "forward_response", err, startTime, routeResult, unifiedModel)
		}

		return err
	}
}

// recordFailure 记录故障到熔断器
func (ps *ProxyService) recordFailure(routeResult *RouteResult, failureType FailureType) {
	if failureType == FailureTypeHard {
		ps.circuitManager.RecordKeyHardFailure(routeResult.Key.ID, routeResult.Channel.ID)
	} else if failureType == FailureTypeSoft {
		ps.circuitManager.RecordKeySoftFailure(routeResult.Key.ID, routeResult.Channel.ID)
	}
}

// GetSupportedModels 获取支持的模型列表
func (ps *ProxyService) GetSupportedModels(ctx context.Context) ([]string, error) {
	// TODO: 实现从数据库查询所有激活的统一模型名
	return []string{}, nil
}

// Close 关闭服务
func (ps *ProxyService) Close() {
	ps.httpClient.Close()
}

// logRequestSuccess 记录成功的请求审计日志
func (ps *ProxyService) logRequestSuccess(
	ctx context.Context,
	traceID string,
	c *gin.Context,
	routeResult *RouteResult,
	statusCode int,
	startTime time.Time,
	isStream bool,
) {
	responseTime := int(time.Since(startTime).Milliseconds())

	log := logger.NewRequestLogBuilder().
		TraceID(traceID).
		AccessToken(getAccessTokenFromContext(c)).
		RequestPath(c.Request.URL.Path).
		RequestMethod(c.Request.Method).
		RequestedModel(routeResult.UnifiedModel).
		UnifiedModel(routeResult.UnifiedModel).
		UpstreamModel(routeResult.UpstreamModel).
		ChannelID(routeResult.Channel.ID).
		ChannelName(routeResult.Channel.Name).
		KeyID(routeResult.Key.ID).
		StatusCode(statusCode).
		ResponseTime(responseTime).
		IsSuccess(true).
		IsStream(isStream).
		ClientIP(c.ClientIP()).
		UserAgent(c.Request.UserAgent()).
		Build()

	// 异步写入数据库
	ps.auditLogger.LogRequestAsync(log)
}

// logRequestError 记录失败的请求审计日志
func (ps *ProxyService) logRequestError(
	ctx context.Context,
	traceID string,
	c *gin.Context,
	errorType string,
	err error,
	startTime time.Time,
	routeResult *RouteResult,
	unifiedModel string,
) {
	responseTime := int(time.Since(startTime).Milliseconds())

	builder := logger.NewRequestLogBuilder().
		TraceID(traceID).
		AccessToken(getAccessTokenFromContext(c)).
		RequestPath(c.Request.URL.Path).
		RequestMethod(c.Request.Method).
		StatusCode(500).
		ResponseTime(responseTime).
		IsSuccess(false).
		ErrorMessage(errorType + ": " + err.Error()).
		ClientIP(c.ClientIP()).
		UserAgent(c.Request.UserAgent())

	// 如果有路由结果，记录完整的渠道和模型信息
	if routeResult != nil {
		builder.RequestedModel(routeResult.UnifiedModel).
			UnifiedModel(routeResult.UnifiedModel).
			UpstreamModel(routeResult.UpstreamModel).
			ChannelID(routeResult.Channel.ID).
			ChannelName(routeResult.Channel.Name).
			KeyID(routeResult.Key.ID)
	} else if unifiedModel != "" {
		// 如果没有路由结果但有模型名，记录模型信息
		builder.RequestedModel(unifiedModel).
			UnifiedModel(unifiedModel)
	}

	log := builder.Build()

	// 异步写入数据库
	ps.auditLogger.LogRequestAsync(log)
}

// GetTraceIDFromContext 从上下文获取 TraceID
func GetTraceIDFromContext(c *gin.Context) string {
	if traceID, exists := c.Get("trace_id"); exists {
		return traceID.(string)
	}
	return "unknown"
}

// getAccessTokenFromContext 从上下文获取访问令牌（脱敏）
func getAccessTokenFromContext(c *gin.Context) string {
	if token, exists := c.Get("access_token_name"); exists {
		return token.(string)
	}
	return ""
}
