package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/circuit"
	configService "github.com/yangshoulai/hydra/internal/service/config"
	"github.com/yangshoulai/hydra/internal/service/logger"
	"github.com/yangshoulai/hydra/internal/service/sniffer"
)

// ProxyService 代理服务主逻辑
type ProxyService struct {
	logger                  *slog.Logger
	loadBalancer            *LoadBalancer
	requestBuilder          *RequestBuilder
	httpClient              *HTTPClient
	responseSniffer         *sniffer.ResponseSniffer
	sseForwarder            *SSEForwarder
	responseForwarder       *ResponseForwarder
	failureClassifier       *FailureClassifier
	retryCoordinator        *RetryCoordinator
	circuitManager          *circuit.Manager
	modelRouter             *ModelRouter
	auditLogger             *logger.AuditLogger // 审计日志记录器
	settingService          *configService.SettingService
	sseForwarderWithSniffer *SSEForwarderWithSniffer // 支持嗅探的SSE转发器
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

	responseSniffer := sniffer.NewResponseSniffer(logger)

	return &ProxyService{
		logger:                  logger,
		loadBalancer:            loadBalancer,
		requestBuilder:          NewRequestBuilder(),
		httpClient:              NewHTTPClient(httpClientConfig, logger),
		responseSniffer:         responseSniffer,
		sseForwarder:            NewSSEForwarder(logger),
		responseForwarder:       NewResponseForwarder(logger),
		failureClassifier:       NewFailureClassifier(),
		retryCoordinator:        NewRetryCoordinator(logger, maxRetries, retryDelay),
		circuitManager:          circuitManager,
		modelRouter:             NewModelRouter(logger),
		auditLogger:             auditLogger,
		settingService:          settingService,
		sseForwarderWithSniffer: NewSSEForwarderWithSniffer(logger, settingService, 1000),
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
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadRequest, "Failed to read request body", traceID)
		ps.logRequestError(ctx, traceID, c, "read_request_body", err, startTime, startTime, nil, "", "", nil, 0)
		return err
	}
	// 重置 Body 使其可以再次读取
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 保存请求 body 字符串用于日志记录
	requestBodyStr := string(bodyBytes)

	// 2. 解析请求获取模型名
	unifiedModel, err := ps.requestBuilder.GetModelFromRequest(bodyBytes)
	if err != nil {
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error(), traceID)
		ps.logRequestError(ctx, traceID, c, "parse_model", err, startTime, startTime, nil, "", requestBodyStr, nil, 0)
		return err
	}

	isStream := ps.requestBuilder.IsStreamRequest(bodyBytes)

	// 根据端点确定端点类型
	endpointType := ps.getEndpointType(endpoint)

	ps.logWithTrace("处理代理请求", traceID,
		slog.String("endpoint", endpoint),
		slog.String("endpoint_type", endpointType),
		slog.String("model", unifiedModel),
		slog.Bool("stream", isStream),
	)

	// 2. 创建重试上下文
	retryCtx := NewRetryContext()

	// 3. 重试循环
	for {
		// 记录本次尝试的开始时间
		attemptStartTime := time.Now()

		// 路由到 Channel 和 Key
		routeResult, err := ps.loadBalancer.RouteWithRetry(
			ctx,
			unifiedModel,
			endpointType,
			ps.retryCoordinator.maxRetries-retryCtx.AttemptCount,
			retryCtx.FailedChannelIDs,
			traceID,
		)

		if err != nil {
			ps.logErrorWithTrace("请求路由失败", traceID, slog.String("model", unifiedModel), slog.String("error", err.Error()))
			ps.responseForwarder.ForwardErrorResponse(c, http.StatusServiceUnavailable, "No available channels for model: "+unifiedModel, traceID)
			ps.logRequestError(ctx, traceID, c, "route_failed", err, startTime, attemptStartTime, nil, unifiedModel, requestBodyStr, nil, 0)
			return err
		}

		// 构建上游请求
		upstreamReq, _, err := ps.requestBuilder.BuildProxyRequest(c, routeResult, endpoint)
		if err != nil {
			ps.logErrorWithTrace("构建代理请求失败", traceID, slog.String("error", err.Error()))
			continue
		}

		// 发送请求
		upstreamResp, err := ps.httpClient.Do(upstreamReq, traceID)

		// 分类故障（综合考虑网络错误和 HTTP 状态码）
		failureType := ps.failureClassifier.ClassifyResponseError(upstreamResp, err)

		// 根据故障类型处理
		if failureType != FailureTypeNone {
			// 404 模型不存在：不记录到熔断器，直接重试
			if failureType != FailureTypeModelNotFound {
				ps.recordFailure(routeResult, failureType)
			}

			// 获取错误信息
			var errorInfo error
			var errorType string
			if err != nil {
				errorInfo = err
				errorType = "network_error"
			} else {
				errorInfo = errors.New("upstream http error")
				errorType = "http_error"
			}

			ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, errorInfo, failureType)

			// 记录这次失败的尝试
			ps.logRequestError(ctx, traceID, c, errorType, errorInfo, startTime, attemptStartTime, routeResult, unifiedModel, requestBodyStr, upstreamResp, retryCtx.AttemptCount)

			if !ps.retryCoordinator.ShouldRetry(retryCtx) {
				ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, "All upstream attempts failed", traceID)
				return errorInfo
			}

			// 等待后重试
			_ = ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
			continue
		}

		// 无故障（包括 200-399，以及不可重试的 4xx 错误）
		// 对于成功的响应，记录到熔断器
		if upstreamResp != nil && upstreamResp.StatusCode < 400 {
			ps.circuitManager.RecordKeySuccess(routeResult.Key.ID, routeResult.Channel.ID)
		}

		// 转发响应
		var responseBodyStr string
		if isStream {
			// 流式响应：使用支持嗅探的转发器（首帧嗅探）
			responseBodyStr, err = ps.sseForwarderWithSniffer.ForwardStreamWithDetection(c, upstreamResp, traceID)

			// 检查是否检测到假200错误
			if fake200Err, ok := err.(*Fake200Error); ok {
				// 首帧检测到假200，视为软故障
				ps.recordFailure(routeResult, FailureTypeSoft)
				ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, fake200Err, FailureTypeSoft)

				ps.logWarnWithTrace("流式响应首帧检测到假200", traceID, slog.String("error_type", fake200Err.Message), slog.String("body_preview", truncateString(fake200Err.Body, 200)))

				// 记录这次失败的尝试
				ps.logRequestError(ctx, traceID, c, "sse_fake_200_first_frame", fake200Err, startTime, attemptStartTime, routeResult, unifiedModel, requestBodyStr, nil, retryCtx.AttemptCount)

				if !ps.retryCoordinator.ShouldRetry(retryCtx) {
					ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, "All upstream attempts failed", traceID)
					return fake200Err
				}

				// 等待后重试
				_ = ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
				continue
			}
		} else {
			// 非流式响应：先完整嗅探，再转发
			sniffResult, sniffErr := ps.responseSniffer.SniffResponse(upstreamResp)
			if sniffErr != nil {
				ps.logErrorWithTrace("嗅探响应失败", traceID, slog.String("error", sniffErr.Error()))
			} else if sniffResult.IsFake200 {
				ps.logWarnWithTrace("检测到假 200 响应", traceID, slog.String("rule", sniffResult.MatchedRule))

				// 视为软故障
				ps.recordFailure(routeResult, FailureTypeSoft)
				ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, errors.New("fake 200 response"), FailureTypeSoft)

				// 记录这次失败的尝试
				ps.logRequestError(ctx, traceID, c, "fake_200_response", errors.New("fake 200 response"), startTime, attemptStartTime, routeResult, unifiedModel, requestBodyStr, nil, retryCtx.AttemptCount)

				if !ps.retryCoordinator.ShouldRetry(retryCtx) {
					ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, "All upstream attempts failed", traceID)
					return errors.New("fake 200 response")
				}

				_ = ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
				continue
			}

			// 转发 JSON 响应
			responseBodyStr, err = ps.responseForwarder.ForwardJSONResponse(c, upstreamResp,
				routeResult.UpstreamModel, routeResult.UnifiedModel, traceID)
		}

		// 记录成功的审计日志
		if err == nil {
			ps.logRequestSuccess(ctx, traceID, c, routeResult, upstreamResp, upstreamResp.StatusCode, startTime, attemptStartTime, isStream, requestBodyStr, responseBodyStr, retryCtx.AttemptCount)
		} else {
			ps.logRequestError(ctx, traceID, c, "forward_response", err, startTime, attemptStartTime, routeResult, unifiedModel, requestBodyStr, upstreamResp, retryCtx.AttemptCount)
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
	upstreamResp *http.Response,
	statusCode int,
	startTime time.Time,
	attemptStartTime time.Time,
	isStream bool,
	requestBody string,
	responseBody string,
	retryCount int,
) {
	// 计算本次尝试的实际响应时间
	attemptResponseTime := int(time.Since(attemptStartTime).Milliseconds())
	// 计算总响应时间（从请求开始到完成）
	totalResponseTime := int(time.Since(startTime).Milliseconds())

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
		ResponseTime(attemptResponseTime).
		IsSuccess(true).
		IsStream(isStream).
		RetryCount(retryCount).
		ClientIP(c.ClientIP()).
		UserAgent(c.Request.UserAgent())

	// 如果有重试，记录总响应时间
	if retryCount > 0 {
		builder.TotalResponseTime(totalResponseTime)
	}

	// 如果调试模式启用，记录完整的请求和响应 body、请求头和响应头
	if ps.auditLogger.IsDebugModeEnabled() {
		// 限制记录长度，避免过大的 body
		const maxBodyLength = 10 * 1024 * 1024 // 10MB

		// 记录请求头
		requestHeaders := headersToJSON(c.Request.Header)
		if requestHeaders != "" {
			builder.RequestHeaders(requestHeaders)
		}

		// 记录响应头
		if upstreamResp != nil {
			responseHeaders := headersToJSON(upstreamResp.Header)
			if responseHeaders != "" {
				builder.ResponseHeaders(responseHeaders)
			}
		}

		// 记录请求体（清洗非 UTF-8 字符）
		if len(requestBody) > 0 {
			cleanBody := sanitizeUTF8(requestBody)
			if len(cleanBody) > maxBodyLength {
				builder.RequestBody(cleanBody[:maxBodyLength] + "...(truncated)")
			} else {
				builder.RequestBody(cleanBody)
			}
		}

		// 记录响应体（清洗非 UTF-8 字符）
		if len(responseBody) > 0 {
			cleanBody := sanitizeUTF8(responseBody)
			if len(cleanBody) > maxBodyLength {
				builder.ResponseBody(cleanBody[:maxBodyLength] + "...(truncated)")
			} else {
				builder.ResponseBody(cleanBody)
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
	attemptStartTime time.Time,
	routeResult *RouteResult,
	unifiedModel string,
	requestBody string,
	upstreamResp *http.Response,
	retryCount int,
) {
	// 计算本次尝试的实际响应时间
	attemptResponseTime := int(time.Since(attemptStartTime).Milliseconds())
	// 计算总响应时间（从请求开始到完成）
	totalResponseTime := int(time.Since(startTime).Milliseconds())

	builder := logger.NewRequestLogBuilder().
		TraceID(traceID).
		AccessToken(getAccessTokenFromContext(c)).
		RequestPath(c.Request.URL.Path).
		RequestMethod(c.Request.Method).
		StatusCode(500).
		ResponseTime(attemptResponseTime).
		IsSuccess(false).
		RetryCount(retryCount).
		ErrorMessage(errorType + ": " + err.Error()).
		ClientIP(c.ClientIP()).
		UserAgent(c.Request.UserAgent())

	// 如果有重试，记录总响应时间
	if retryCount > 0 {
		builder.TotalResponseTime(totalResponseTime)
	}

	// 如果有路由结果，记录完整的渠道和模型信息
	if routeResult != nil {
		builder.RequestedModel(routeResult.UnifiedModel).
			UnifiedModel(routeResult.UnifiedModel).
			UpstreamModel(routeResult.UpstreamModel).
			ChannelID(routeResult.Channel.ID).
			ChannelName(routeResult.Channel.Name).
			KeyID(routeResult.Key.ID)

		// 如果有上游响应，使用实际的状态码
		if upstreamResp != nil {
			builder.StatusCode(upstreamResp.StatusCode)
		}
	} else if unifiedModel != "" {
		// 如果没有路由结果但有模型名，记录模型信息
		builder.RequestedModel(unifiedModel).
			UnifiedModel(unifiedModel)
	}

	// 如果调试模式启用，记录请求 body、请求头和响应头
	if ps.auditLogger.IsDebugModeEnabled() {
		// 记录请求头
		requestHeaders := headersToJSON(c.Request.Header)
		if requestHeaders != "" {
			builder.RequestHeaders(requestHeaders)
		}

		// 记录响应头（如果有上游响应）
		if upstreamResp != nil {
			responseHeaders := headersToJSON(upstreamResp.Header)
			if responseHeaders != "" {
				builder.ResponseHeaders(responseHeaders)
			}
		}

		// 记录请求体（清洗非 UTF-8 字符）
		const maxBodyLength = 10 * 1024 * 1024 // 10MB
		if len(requestBody) > 0 {
			cleanBody := sanitizeUTF8(requestBody)
			if len(cleanBody) > maxBodyLength {
				builder.RequestBody(cleanBody[:maxBodyLength] + "...(truncated)")
			} else {
				builder.RequestBody(cleanBody)
			}
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
	return ""
}

// logWithTrace 记录带有 trace_id 的 Info 日志
func (ps *ProxyService) logWithTrace(msg string, traceID string, args ...slog.Attr) {
	allArgs := append([]slog.Attr{slog.String("trace_id", traceID)}, args...)
	// 将 slog.Attr 转换为 []any
	anyArgs := make([]any, len(allArgs))
	for i, attr := range allArgs {
		anyArgs[i] = attr
	}
	ps.logger.Log(context.Background(), slog.LevelInfo, msg, anyArgs...)
}

// logWarnWithTrace 记录带有 trace_id 的 Warn 日志
func (ps *ProxyService) logWarnWithTrace(msg string, traceID string, args ...slog.Attr) {
	allArgs := append([]slog.Attr{slog.String("trace_id", traceID)}, args...)
	// 将 slog.Attr 转换为 []any
	anyArgs := make([]any, len(allArgs))
	for i, attr := range allArgs {
		anyArgs[i] = attr
	}
	ps.logger.Log(context.Background(), slog.LevelWarn, msg, anyArgs...)
}

// logErrorWithTrace 记录带有 trace_id 的 Error 日志
func (ps *ProxyService) logErrorWithTrace(msg string, traceID string, args ...slog.Attr) {
	allArgs := append([]slog.Attr{slog.String("trace_id", traceID)}, args...)
	// 将 slog.Attr 转换为 []any
	anyArgs := make([]any, len(allArgs))
	for i, attr := range allArgs {
		anyArgs[i] = attr
	}
	ps.logger.Log(context.Background(), slog.LevelError, msg, anyArgs...)
}

// logDebugWithTrace 记录带有 trace_id 的 Debug 日志
func (ps *ProxyService) logDebugWithTrace(msg string, traceID string, args ...slog.Attr) {
	allArgs := append([]slog.Attr{slog.String("trace_id", traceID)}, args...)
	// 将 slog.Attr 转换为 []any
	anyArgs := make([]any, len(allArgs))
	for i, attr := range allArgs {
		anyArgs[i] = attr
	}
	ps.logger.Log(context.Background(), slog.LevelDebug, msg, anyArgs...)
}

// getAccessTokenFromContext 从上下文获取访问令牌（脱敏）
func getAccessTokenFromContext(c *gin.Context) string {
	if token, exists := c.Get("access_token_name"); exists {
		return token.(string)
	}
	return ""
}

// headersToJSON 将 HTTP 头转换为 JSON 字符串
func headersToJSON(headers http.Header) string {
	if headers == nil {
		return ""
	}

	// 转换为 map
	headerMap := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			headerMap[key] = values[0] // 只取第一个值
		}
	}

	// 转换为 JSON
	jsonBytes, err := json.Marshal(headerMap)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}

// OnConfigChanged 实现 ConfigListener 接口，响应配置变更
func (ps *ProxyService) OnConfigChanged(ctx context.Context, category string) {
	// 处理 proxy 分类的配置变更
	if category == "proxy" {
		requestTimeout, _, _, maxRetry := ps.settingService.GetProxyConfig(ctx)
		retryDelay := 500 * time.Millisecond // 默认值

		// 更新 HTTPClient 的超时时间
		ps.httpClient.UpdateRequestTimeout(requestTimeout)

		// 更新 RetryCoordinator 的配置
		ps.retryCoordinator.UpdateConfig(maxRetry, retryDelay)

		ps.logger.Info("代理服务配置已更新",
			slog.Duration("request_timeout", requestTimeout),
			slog.Int("max_retry", maxRetry),
			slog.Duration("retry_delay", retryDelay),
		)
	}

	// 处理 sniffer 分类的配置变更
	if category == "sniffer" {
		// 更新流式错误关键词
		keywords := ps.settingService.GetStreamErrorRules(ctx)
		ps.responseSniffer.UpdatePlainTextErrorKeywords(keywords)
		ps.sseForwarderWithSniffer.UpdateErrorKeywords(keywords)

		ps.logger.Info("流式错误关键词配置已更新", slog.Int("keywords_count", len(keywords)))
	}
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

// sanitizeUTF8 清洗字符串中的非 UTF-8 字符
// 将非法的 UTF-8 字节序列替换为替换字符，避免数据库写入失败
func sanitizeUTF8(data string) string {
	if utf8.ValidString(data) {
		return data // 数据是有效的 UTF-8，直接返回
	}

	// 包含无效的 UTF-8 序列，需要清洗
	var buf strings.Builder
	buf.Grow(len(data))

	for i, r := range data {
		if r == utf8.RuneError {
			// 检查是否真的是非法字符
			_, size := utf8.DecodeRuneInString(data[i:])
			if size == 1 {
				// 非法的 UTF-8 字节，替换为 �
				buf.WriteRune(utf8.RuneError)
			} else {
				buf.WriteRune(r)
			}
		} else {
			buf.WriteRune(r)
		}
	}

	return buf.String()
}
