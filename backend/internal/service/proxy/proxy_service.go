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
	configService "github.com/yangshoulai/hydra/internal/service/config"
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
	settingService    *configService.SettingService
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
	settingService *configService.SettingService,
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
		settingService:    settingService,
	}
}

// ProxyChatCompletions 代理 Chat Completions 请求
func (ps *ProxyService) ProxyChatCompletions(c *gin.Context) error {
	return ps.proxyRequest(c, "/v1/chat/completions")
}

// ProxyResponses 代理 Responses 请求
func (ps *ProxyService) ProxyResponses(c *gin.Context) error {
	return ps.proxyRequest(c, "/v1/responses")
}

// ProxyMessages 代理 Anthropic Messages 请求
func (ps *ProxyService) ProxyMessages(c *gin.Context) error {
	return ps.proxyRequest(c, "/v1/messages")
}

// proxyRequest 通用代理请求处理
func (ps *ProxyService) proxyRequest(c *gin.Context, endpoint string) error {
	ctx := c.Request.Context()
	startTime := time.Now()
	traceID := GetTraceIDFromContext(c)

	// 1. 读取请求 Body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadRequest, "Failed to read request body")
		ps.logRequestError(ctx, traceID, c, "read_request_body", err, startTime, nil, "", "")
		return err
	}
	// 重置 Body 使其可以再次读取
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 保存请求 body 字符串用于日志记录
	requestBodyStr := string(bodyBytes)

	// 2. 解析请求获取模型名
	unifiedModel, err := ps.requestBuilder.GetModelFromRequest(bodyBytes)
	if err != nil {
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		ps.logRequestError(ctx, traceID, c, "parse_model", err, startTime, nil, "", requestBodyStr)
		return err
	}

	isStream := ps.requestBuilder.IsStreamRequest(bodyBytes)

	// 根据端点确定端点类型
	endpointType := ps.getEndpointType(endpoint)

	ps.logger.Info("处理代理请求",
		slog.String("endpoint", endpoint),
		slog.String("endpoint_type", endpointType),
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
			endpointType,
			ps.retryCoordinator.maxRetries-retryCtx.AttemptCount,
			retryCtx.FailedChannelIDs,
		)

		if err != nil {
			ps.logger.Error("请求路由失败",
				slog.String("model", unifiedModel),
				slog.String("error", err.Error()),
			)
			ps.responseForwarder.ForwardErrorResponse(c, http.StatusServiceUnavailable,
				"No available channels for model: "+unifiedModel)
			ps.logRequestError(ctx, traceID, c, "route_failed", err, startTime, nil, unifiedModel, requestBodyStr)
			return err
		}

		// 构建上游请求
		upstreamReq, _, err := ps.requestBuilder.BuildProxyRequest(c, routeResult, endpoint)
		if err != nil {
			ps.logger.Error("构建代理请求失败", slog.String("error", err.Error()))
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
			ps.logRequestError(ctx, traceID, c, "upstream_request_failed", err, startTime, routeResult, unifiedModel, requestBodyStr)

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
				ps.logger.Error("嗅探响应失败",
					slog.String("error", err.Error()),
				)
			} else if sniffResult.IsFake200 {
				ps.logger.Warn("检测到假 200 响应",
					slog.String("rule", sniffResult.MatchedRule),
				)

				// 视为软故障
				ps.recordFailure(routeResult, FailureTypeSoft)
				ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID,
					errors.New("fake 200 response"), FailureTypeSoft)

				// 记录这次失败的尝试
				ps.logRequestError(ctx, traceID, c, "fake_200_response", errors.New("fake 200 response"), startTime, routeResult, unifiedModel, requestBodyStr)

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
				ps.logRequestError(ctx, traceID, c, "upstream_http_error", errors.New("upstream error"), startTime, routeResult, unifiedModel, requestBodyStr)

				if ps.retryCoordinator.ShouldRetry(retryCtx) {
					ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
					continue
				}
			} else {
				// 不可重试的错误（如 400, 401, 403 等），也需要记录审计日志
				ps.logRequestError(ctx, traceID, c, "upstream_http_error_non_retryable",
					errors.New("upstream error"), startTime, routeResult, unifiedModel, requestBodyStr)
			}

			// 转发错误响应
			_, err = ps.responseForwarder.ForwardResponse(c, upstreamResp)
			return err
		}

		// 成功响应
		ps.circuitManager.RecordKeySuccess(routeResult.Key.ID, routeResult.Channel.ID)

		// 转发响应
		var responseBodyStr string
		if isStream {
			// 流式响应：记录流式内容
			responseBodyStr, err = ps.sseForwarder.ForwardStreamWithCapture(c, upstreamResp)
		} else {
			// 非流式响应：ForwardJSONResponse 返回响应 body
			responseBodyStr, err = ps.responseForwarder.ForwardJSONResponse(c, upstreamResp,
				routeResult.UpstreamModel, routeResult.UnifiedModel)
		}

		// 记录成功的审计日志
		if err == nil {
			ps.logRequestSuccess(ctx, traceID, c, routeResult, upstreamResp.StatusCode, startTime, isStream, requestBodyStr, responseBodyStr)
		} else {
			ps.logRequestError(ctx, traceID, c, "forward_response", err, startTime, routeResult, unifiedModel, requestBodyStr)
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

// UpdateSnifferKeywords 更新嗅探器的明文错误关键词
func (ps *ProxyService) UpdateSnifferKeywords(keywords []string) {
	ps.responseSniffer.UpdatePlainTextErrorKeywords(keywords)
	ps.logger.Info("嗅探器关键词已更新", "count", len(keywords))
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
	requestBody string,
	responseBody string,
) {
	responseTime := int(time.Since(startTime).Milliseconds())

	builder := logger.NewRequestLogBuilder().
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
		UserAgent(c.Request.UserAgent())

	// 如果调试模式启用，记录完整的请求和响应 body
	if ps.auditLogger.IsDebugModeEnabled() {
		// 限制记录长度，避免过大的 body
		const maxBodyLength = 10240 // 10KB

		if len(requestBody) > 0 {
			if len(requestBody) > maxBodyLength {
				builder.RequestBody(requestBody[:maxBodyLength] + "...(truncated)")
			} else {
				builder.RequestBody(requestBody)
			}
		}

		if len(responseBody) > 0 {
			if len(responseBody) > maxBodyLength {
				builder.ResponseBody(responseBody[:maxBodyLength] + "...(truncated)")
			} else {
				builder.ResponseBody(responseBody)
			}
		}
	}

	log := builder.Build()

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
	requestBody string,
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

	// 如果调试模式启用，记录请求 body
	if ps.auditLogger.IsDebugModeEnabled() && len(requestBody) > 0 {
		const maxBodyLength = 10240 // 10KB
		if len(requestBody) > maxBodyLength {
			builder.RequestBody(requestBody[:maxBodyLength] + "...(truncated)")
		} else {
			builder.RequestBody(requestBody)
		}
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

// OnConfigChanged 实现 ConfigListener 接口，响应配置变更
func (ps *ProxyService) OnConfigChanged(ctx context.Context, category string) {
	// 只处理 proxy 分类的配置变更
	if category != "proxy" {
		return
	}

	// 从配置服务获取最新的代理配置
	_, _, _, maxRetry := ps.settingService.GetProxyConfig(ctx)
	retryDelay := 500 * time.Millisecond // 默认值

	// 更新 RetryCoordinator 的配置
	ps.retryCoordinator.UpdateConfig(maxRetry, retryDelay)

	ps.logger.Info("代理服务配置已更新",
		slog.Int("max_retry", maxRetry),
		slog.Duration("retry_delay", retryDelay),
	)
}

// getEndpointType 根据端点路径确定端点类型
func (ps *ProxyService) getEndpointType(endpoint string) string {
	switch endpoint {
	case "/v1/chat/completions":
		return "openai"
	case "/v1/responses":
		return "openai-response"
	case "/v1/messages":
		return "anthropic"
	default:
		return "openai" // 默认为 openai
	}
}
