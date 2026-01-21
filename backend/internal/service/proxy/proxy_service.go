package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/endpoint"
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

// ProxyRequest 通用代理请求入口
func (ps *ProxyService) ProxyRequest(c *gin.Context, endpointPath string) error {
	return ps.proxyRequest(c, endpointPath)
}

// proxyRequest 通用代理请求处理
func (ps *ProxyService) proxyRequest(c *gin.Context, endpoint string) error {
	ctx := c.Request.Context()
	startTime := time.Now()
	traceID := GetTraceIDFromContext(c)

	endpointType := ps.getEndpointType(endpoint)
	mainLog := logger.NewMainLogBuilder().
		TraceID(traceID).
		EndpointType(endpointType).
		RequestPath(c.Request.URL.Path).
		RequestMethod(c.Request.Method).
		AccessToken(getAccessTokenFromContext(c)).
		ClientIP(c.ClientIP()).
		UserAgent(c.Request.UserAgent()).
		StartTime(startTime)

	// 1. 读取请求 Body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadRequest, "Failed to read request body", traceID)
		mainLog.EndTime(time.Now()).StatusCode(http.StatusBadRequest).ErrorMessage("读取请求异常: " + err.Error())
		ps.auditLogger.LogAsync(mainLog.Build())
		return err
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	requestBodyStr := string(bodyBytes)

	// 2. 解析请求获取模型名
	unifiedModel, err := ps.requestBuilder.GetModelFromRequest(bodyBytes)
	if err != nil {
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error(), traceID)
		mainLog.EndTime(time.Now()).StatusCode(http.StatusBadRequest).ErrorMessage("获取请求模型异常: " + err.Error())
		ps.auditLogger.LogAsync(mainLog.Build())
		return err
	}

	isStream := ps.requestBuilder.IsStreamRequest(bodyBytes)
	mainLog.RequestedModel(unifiedModel).IsStream(isStream)

	ps.logWithTrace("处理请求", traceID,
		slog.String("endpoint", endpoint),
		slog.String("endpoint_type", endpointType),
		slog.String("model", unifiedModel),
		slog.Bool("stream", isStream),
	)

	// 3. 重试循环
	retryCtx := NewRetryContext()

	for {
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
			ps.logErrorWithTrace("路由失败", traceID, slog.String("model", unifiedModel), slog.String("error", err.Error()))
			ps.responseForwarder.ForwardErrorResponse(c, http.StatusServiceUnavailable, "No available channels for model: "+unifiedModel, traceID)
			mainLog.EndTime(time.Now()).StatusCode(http.StatusServiceUnavailable).ErrorMessage("没有可用渠道")
			ps.auditLogger.LogAsync(mainLog.Build())
			return err
		}
		ps.logWithTrace("路由成功", traceID, slog.Uint64("channel_id", uint64(routeResult.Channel.ID)), slog.String("channel_name", routeResult.Channel.Name), slog.Uint64("key_id", uint64(routeResult.Key.ID)), slog.String("unified_model", unifiedModel), slog.String("upstream_model", routeResult.UpstreamModel))

		// 构建上游请求
		upstreamReq, _, err := ps.requestBuilder.BuildProxyRequest(c, routeResult, endpoint)

		detailLog := logger.NewDetailLogBuilder().
			ChannelID(routeResult.Channel.ID).
			ChannelName(routeResult.Channel.Name).
			Model(routeResult.UpstreamModel).
			KeyID(routeResult.Key.ID).
			StartTime(attemptStartTime).
			RequestBody(requestBodyStr).
			RetryIndex(retryCtx.AttemptCount).
			IsStream(isStream)

		if err != nil {
			ps.logErrorWithTrace("构建代理请求失败", traceID, slog.String("error", err.Error()))
			ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, err, FailureTypeHard)

			detailLog.StatusCode(http.StatusInternalServerError).
				IsSuccess(false).
				Status("failed").
				ErrorMessage("构建代理请求异常" + err.Error()).
				EndTime(time.Now()).Duration(int(time.Now().Sub(attemptStartTime).Milliseconds()))

			mainLog.AddDetail(detailLog)
			if !ps.retryCoordinator.ShouldRetry(retryCtx) {
				ps.responseForwarder.ForwardErrorResponse(c, http.StatusInternalServerError, "Failed to build proxy request", traceID)
				mainLog.EndTime(time.Now()).Duration(int(time.Now().Sub(startTime))).StatusCode(http.StatusInternalServerError).ErrorMessage("构建代理请求异常" + err.Error()).LastChannelID(routeResult.Channel.ID).LastChannelName(routeResult.Channel.Name).LastModel(routeResult.UpstreamModel)
				ps.auditLogger.LogAsync(mainLog.Build())
				return err
			}

			_ = ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
			continue
		}
		detailLog.RequestHeaders(headersToJSON(upstreamReq.Header))
		// 发送请求
		upstreamResp, err := ps.httpClient.Do(upstreamReq, traceID)

		if upstreamResp != nil {
			detailLog.StatusCode(upstreamResp.StatusCode).ResponseHeaders(headersToJSON(upstreamResp.Header))
		}

		// 分类故障
		failureType, errMsg := ps.failureClassifier.ClassifyResponseError(upstreamResp, err)

		// 处理故障
		if failureType != FailureTypeNone {
			if failureType != FailureTypeModelNotFound {
				ps.recordFailure(routeResult, failureType)
			}

			detailLog.IsSuccess(false).Status("failed").ErrorMessage(errMsg).EndTime(time.Now()).Duration(int(time.Now().Sub(attemptStartTime).Milliseconds()))
			ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, errors.New(errMsg), failureType)
			mainLog.AddDetail(detailLog)
			if !ps.retryCoordinator.ShouldRetry(retryCtx) {
				ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, "All upstream attempts failed", traceID)
				statusCode := http.StatusBadGateway
				if upstreamResp != nil {
					statusCode = upstreamResp.StatusCode
				}
				mainLog.EndTime(time.Now()).Duration(int(time.Now().Sub(startTime))).StatusCode(statusCode).ErrorMessage(errMsg).LastChannelID(routeResult.Channel.ID).LastChannelName(routeResult.Channel.Name).LastModel(routeResult.UpstreamModel)
				ps.auditLogger.LogAsync(mainLog.Build())
				return errors.New(errMsg)
			}

			_ = ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
			continue
		}

		// 成功响应
		if upstreamResp != nil && upstreamResp.StatusCode < 400 {
			ps.circuitManager.RecordKeySuccess(routeResult.Key.ID, routeResult.Channel.ID)
		}

		detailLog.IsSuccess(true).Status("success")

		// 转发响应
		var responseBodyStr string
		var streamChunks int
		var firstChunkTime int
		var forwardErr error

		if isStream {
			responseBodyStr, streamChunks, firstChunkTime, forwardErr = ps.sseForwarderWithSniffer.ForwardStreamWithDetection(c, upstreamResp, traceID)
			// 检查假200错误
			if fake200Err, ok := forwardErr.(*Fake200Error); ok {
				ps.recordFailure(routeResult, FailureTypeSoft)
				ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, fake200Err, FailureTypeSoft)
				ps.logWarnWithTrace("检测到流式假 200 响应", traceID, slog.String("error_type", fake200Err.Message))

				detailLog.IsSuccess(false).Status("failed").
					StreamChunks(streamChunks).
					StreamFirstChunkTime(firstChunkTime).
					ResponseBody(responseBodyStr).
					ErrorMessage(fake200Err.Error()).
					EndTime(time.Now()).EndTime(time.Now()).Duration(int(time.Now().Sub(attemptStartTime).Milliseconds()))
				mainLog.AddDetail(detailLog)

				if !ps.retryCoordinator.ShouldRetry(retryCtx) {
					ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, "All upstream attempts failed", traceID)
					mainLog.EndTime(time.Now()).Duration(int(time.Now().Sub(startTime))).StatusCode(http.StatusBadGateway).ErrorMessage(fake200Err.Error()).LastChannelID(routeResult.Channel.ID).LastChannelName(routeResult.Channel.Name).LastModel(routeResult.UpstreamModel)
					ps.auditLogger.LogAsync(mainLog.Build())
					return fake200Err
				}

				_ = ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
				continue
			}
		} else {
			sniffResult, sniffErr := ps.responseSniffer.SniffResponse(upstreamResp)
			if sniffErr != nil {
				ps.logErrorWithTrace("嗅探响应失败", traceID, slog.String("error", sniffErr.Error()))
			} else if sniffResult.IsFake200 {
				ps.logWarnWithTrace("检测到假 200 响应", traceID, slog.String("rule", sniffResult.MatchedRule))
				ps.recordFailure(routeResult, FailureTypeSoft)
				ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, errors.New("fake 200 response"), FailureTypeSoft)

				detailLog.EndTime(time.Now()).EndTime(time.Now()).Duration(int(time.Now().Sub(attemptStartTime).Milliseconds())).IsSuccess(false).Status("failed").ResponseBody(string(sniffResult.Body))
				mainLog.AddDetail(detailLog)

				if !ps.retryCoordinator.ShouldRetry(retryCtx) {
					ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, "All upstream attempts failed", traceID)
					mainLog.EndTime(time.Now()).Duration(int(time.Now().Sub(startTime))).StatusCode(http.StatusBadGateway).ErrorMessage("Fake 200 response").LastChannelID(routeResult.Channel.ID).LastChannelName(routeResult.Channel.Name).LastModel(routeResult.UpstreamModel)
					return errors.New("fake 200 response")
				}

				_ = ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
				continue
			}

			responseBodyStr, forwardErr = ps.responseForwarder.ForwardJSONResponse(c, upstreamResp, routeResult.UpstreamModel, routeResult.UnifiedModel, traceID)
		}

		// 更新明细日志
		errorMsg := ""
		if forwardErr != nil {
			if errors.Is(forwardErr, context.Canceled) {
				errorMsg = "渠道断开连接"
			} else {
				errorMsg = forwardErr.Error()
			}
		}
		detailLog.ResponseBody(responseBodyStr).StreamChunks(streamChunks).StreamFirstChunkTime(firstChunkTime).
			IsSuccess(forwardErr == nil).
			Status("failed").
			ErrorMessage(errorMsg).EndTime(time.Now()).Duration(int(time.Now().Sub(attemptStartTime).Milliseconds()))

		mainLog.AddDetail(detailLog)
		statusCode := upstreamResp.StatusCode
		mainLog.IsSuccess(forwardErr == nil).EndTime(time.Now()).Duration(int(time.Now().Sub(startTime))).StatusCode(statusCode).ErrorMessage(errorMsg).LastChannelID(routeResult.Channel.ID).LastChannelName(routeResult.Channel.Name).LastModel(routeResult.UpstreamModel)
		ps.auditLogger.LogAsync(mainLog.Build())
		return forwardErr
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

// UpdateSnifferKeywords 更新嗅探器的明文错误关键词
func (ps *ProxyService) UpdateSnifferKeywords(keywords []string) {
	ps.responseSniffer.UpdatePlainTextErrorKeywords(keywords)
	ps.logger.Info("嗅探器关键词已更新", "count", len(keywords))
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
func (ps *ProxyService) getEndpointType(endpointPath string) string {
	// 从端点注册中心查找匹配的端点
	for _, ep := range endpoint.GetAll() {
		if ep.GetPath() == endpointPath {
			return ep.GetType()
		}
	}

	// 如果找不到，返回默认的 openai
	ps.logger.Warn("未找到端点类型，使用默认值",
		slog.String("endpoint_path", endpointPath),
		slog.String("default_type", "openai"),
	)
	return "openai"
}
