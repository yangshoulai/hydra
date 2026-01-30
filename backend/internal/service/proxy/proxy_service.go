package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

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
	auditLogger             *logger.AuditLogger // 审计日志记录器
	requestLogRepo          *repository.RequestLogRepository
	keyRepo                 *repository.KeyRepository
	modelConfigRepo         *repository.ChannelModelConfigRepository
	accessTokenRepo         *repository.AccessTokenRepository
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
	requestLogRepo *repository.RequestLogRepository,
	keyRepo *repository.KeyRepository,
	modelConfigRepo *repository.ChannelModelConfigRepository,
	accessTokenRepo *repository.AccessTokenRepository,
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
		auditLogger:             auditLogger,
		requestLogRepo:          requestLogRepo,
		keyRepo:                 keyRepo,
		modelConfigRepo:         modelConfigRepo,
		accessTokenRepo:         accessTokenRepo,
		settingService:          settingService,
		sseForwarderWithSniffer: NewSSEForwarderWithSniffer(logger, settingService, 1000),
	}
}

// ProxyRequest 通用代理请求入口
func (ps *ProxyService) ProxyRequest(c *gin.Context, endpointPath string) error {
	return ps.proxyRequest(c, endpointPath)
}

// proxyRequest 通用代理请求处理
func (ps *ProxyService) proxyRequest(c *gin.Context, endpointPath string) error {
	ctx := c.Request.Context()
	startTime := time.Now()
	traceID := GetTraceIDFromContext(c)

	endpointType, err := ps.getEndpointType(endpointPath)
	if err != nil {
		return err
	}
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
		ps.logErrorWithTrace("读取请求异常", traceID, slog.String("error", err.Error()))
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadRequest, "Failed to read request body", traceID)
		mainLog.EndTime(time.Now()).StatusCode(http.StatusBadRequest).ErrorMessage("读取请求异常: " + err.Error())
		ps.auditLogger.LogAsync(mainLog.Build())
		return err
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	requestBodyStr := string(bodyBytes)

	// 2. 解析请求获取模型名
	unifiedModel, err := ps.requestBuilder.GetModelFromRequest(bodyBytes, endpointType, c.Request.URL.Path)
	if err != nil {
		ps.logErrorWithTrace("获取请求模型异常", traceID, slog.String("error", err.Error()))
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error(), traceID)
		mainLog.EndTime(time.Now()).StatusCode(http.StatusBadRequest).ErrorMessage("获取请求模型异常: " + err.Error())
		ps.auditLogger.LogAsync(mainLog.Build())
		return err
	}

	if ps.modelConfigRepo != nil {
		supported, validateErr := ps.modelConfigRepo.ExistsActiveUnifiedModel(ctx, unifiedModel, endpointType)
		if validateErr != nil {
			ps.logErrorWithTrace("校验模型异常", traceID, slog.String("error", validateErr.Error()))
			ps.responseForwarder.ForwardErrorResponse(c, http.StatusInternalServerError, "Failed to validate model", traceID)
			mainLog.EndTime(time.Now()).StatusCode(http.StatusInternalServerError).ErrorMessage("校验模型异常: " + validateErr.Error())
			ps.auditLogger.LogAsync(mainLog.Build())
			return validateErr
		}
		if !supported {
			ps.responseForwarder.ForwardErrorResponse(c, http.StatusNotFound, "Model not found: "+unifiedModel, traceID)
			return ErrModelNotFound
		}
	}

	isStream := ps.requestBuilder.IsStreamRequest(bodyBytes, endpointType, c.Request.URL.Path)
	mainLog.RequestedModel(unifiedModel).IsStream(isStream)

	ps.logWithTrace("处理请求", traceID,
		slog.String("endpoint", endpointPath),
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
			retryCtx.FailedModelIDs,
			retryCtx.FailedKeyIDs,
			traceID,
		)

		if err != nil {
			ps.logErrorWithTrace("路由异常", traceID, slog.String("model", unifiedModel), slog.String("error", err.Error()))
			ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, "Error when route model: "+unifiedModel, traceID)
			mainLog.EndTime(time.Now()).StatusCode(http.StatusBadGateway).ErrorMessage(err.Error())
			ps.auditLogger.LogAsync(mainLog.Build())
			return err
		}
		ps.logWithTrace("路由成功", traceID, slog.Uint64("channel_id", uint64(routeResult.Channel.ID)), slog.String("channel_name", routeResult.Channel.Name), slog.Uint64("key_id", uint64(routeResult.Key.ID)), slog.Uint64("model_config_id", uint64(routeResult.ModelConfigID)), slog.String("unified_model", unifiedModel), slog.String("upstream_model", routeResult.UpstreamModel))

		// 构建上游请求
		upstreamReq, _, err := ps.requestBuilder.BuildProxyRequest(c, routeResult, endpointPath)

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
			ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, routeResult.Channel.Name, routeResult.ModelConfigID, routeResult.Key.ID, err, FailureTypeHard)

			detailLog.StatusCode(http.StatusInternalServerError).
				IsSuccess(false).
				Status("failed").
				ErrorMessage("构建代理请求异常" + err.Error()).
				EndTime(time.Now()).Duration(int(time.Now().Sub(attemptStartTime).Milliseconds()))

			mainLog.AddDetail(detailLog)
			mainLog.EndTime(time.Now()).Duration(int(time.Now().Sub(startTime))).StatusCode(http.StatusInternalServerError).ErrorMessage("构建代理请求异常" + err.Error()).LastChannelID(routeResult.Channel.ID).LastChannelName(routeResult.Channel.Name).LastModel(routeResult.UpstreamModel)
			if !ps.retryCoordinator.ShouldRetry(retryCtx) {
				ps.responseForwarder.ForwardErrorResponse(c, http.StatusInternalServerError, "Failed to build proxy request", traceID)
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
		failureType, failureScope, errMsg := ps.failureClassifier.ClassifyResponseError(upstreamResp, err)

		// 处理故障
		if failureType != FailureTypeNone {
			respBody, readRespErr := readAndResetResponseBody(upstreamResp)
			if readRespErr != nil {
				ps.logWarnWithTrace("读取响应体失败", traceID, slog.String("error", readRespErr.Error()))
			}
			statusCode := -1
			if upstreamResp != nil {
				statusCode = upstreamResp.StatusCode
			}
			ps.logErrorWithTrace("渠道故障", traceID, slog.String("failure_type", string(failureType)),
				slog.Uint64("channel_id", uint64(routeResult.Channel.ID)),
				slog.String("channel_name", routeResult.Channel.Name),
				slog.String("model", routeResult.UnifiedModel),
				slog.String("upstream_model", routeResult.UpstreamModel),
				slog.String("error", errMsg),
				slog.String("url", upstreamReq.URL.String()),
				slog.Int("http_status", statusCode),
				slog.String("response_body", string(respBody)),
			)
			ps.recordFailure(routeResult, failureType, failureScope, errMsg)

			detailLog.IsSuccess(false).Status("failed").ResponseBody(string(respBody)).ErrorMessage(errMsg).EndTime(time.Now()).Duration(int(time.Now().Sub(attemptStartTime).Milliseconds()))
			ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, routeResult.Channel.Name, routeResult.ModelConfigID, routeResult.Key.ID, errors.New(errMsg), failureType)
			mainLog.AddDetail(detailLog)
			mainLog.EndTime(time.Now()).Duration(int(time.Now().Sub(startTime))).StatusCode(http.StatusBadGateway).ErrorMessage(errMsg).LastChannelID(routeResult.Channel.ID).LastChannelName(routeResult.Channel.Name).LastModel(routeResult.UpstreamModel)
			if !ps.retryCoordinator.ShouldRetry(retryCtx) {
				ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, "All upstream attempts failed", traceID)
				ps.auditLogger.LogAsync(mainLog.Build())
				return errors.New(errMsg)
			}

			_ = ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
			continue
		}

		// 成功响应
		if upstreamResp != nil && upstreamResp.StatusCode < 400 {
			ps.recordSuccess(routeResult)
		}

		detailLog.IsSuccess(true).Status("success")

		// 转发响应
		var responseBodyStr string
		var streamChunks int
		var firstChunkTime int
		var forwardErr error

		if isStream {
			responseBodyStr, streamChunks, firstChunkTime, forwardErr = ps.sseForwarderWithSniffer.ForwardStreamWithDetection(c, upstreamResp, traceID)
			if emptyBodyErr, ok := forwardErr.(*EmptySSEBodyError); ok {
				ps.recordFailure(routeResult, FailureTypeSoft, FailureScopeNone, forwardErr.Error())
				ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, routeResult.Channel.Name, routeResult.ModelConfigID, routeResult.Key.ID, emptyBodyErr, FailureTypeSoft)
				ps.logWarnWithTrace("检测到空的流式响应体", traceID, slog.String("error", emptyBodyErr.Message))

				detailLog.IsSuccess(false).Status("failed").
					StreamChunks(streamChunks).
					StreamFirstChunkTime(firstChunkTime).
					ResponseBody(responseBodyStr).
					ErrorMessage(emptyBodyErr.Error()).
					EndTime(time.Now()).Duration(int(time.Now().Sub(attemptStartTime).Milliseconds()))
				mainLog.AddDetail(detailLog)
				mainLog.EndTime(time.Now()).Duration(int(time.Now().Sub(startTime))).StatusCode(http.StatusBadGateway).ErrorMessage(emptyBodyErr.Error()).LastChannelID(routeResult.Channel.ID).LastChannelName(routeResult.Channel.Name).LastModel(routeResult.UpstreamModel)
				if !ps.retryCoordinator.ShouldRetry(retryCtx) {
					ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, "All upstream attempts failed", traceID)
					ps.auditLogger.LogAsync(mainLog.Build())
					return emptyBodyErr
				}

				_ = ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
				continue
			}
			// 检查假200错误
			if fake200Err, ok := forwardErr.(*Fake200Error); ok {
				ps.recordFailure(routeResult, FailureTypeSoft, FailureScopeKey, forwardErr.Error())
				ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, routeResult.Channel.Name, routeResult.ModelConfigID, routeResult.Key.ID, fake200Err, FailureTypeSoft)
				ps.logWarnWithTrace("检测到流式假 200 响应", traceID, slog.String("error", fake200Err.Message))

				detailLog.IsSuccess(false).Status("failed").
					StreamChunks(streamChunks).
					StreamFirstChunkTime(firstChunkTime).
					ResponseBody(responseBodyStr).
					ErrorMessage(fake200Err.Error()).
					EndTime(time.Now()).EndTime(time.Now()).Duration(int(time.Now().Sub(attemptStartTime).Milliseconds()))
				mainLog.AddDetail(detailLog)
				mainLog.EndTime(time.Now()).Duration(int(time.Now().Sub(startTime))).StatusCode(http.StatusBadGateway).ErrorMessage(fake200Err.Error()).LastChannelID(routeResult.Channel.ID).LastChannelName(routeResult.Channel.Name).LastModel(routeResult.UpstreamModel)
				if !ps.retryCoordinator.ShouldRetry(retryCtx) {
					ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, "All upstream attempts failed", traceID)
					ps.auditLogger.LogAsync(mainLog.Build())
					return fake200Err
				}

				_ = ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
				continue
			}
		} else {
			sniffResult, sniffErr := ps.responseSniffer.SniffResponse(upstreamResp)
			if sniffErr != nil {
				ps.logWarnWithTrace("嗅探响应失败", traceID, slog.String("error", sniffErr.Error()))
			} else if sniffResult.IsFake200 {
				ps.logErrorWithTrace("检测到假 200 响应", traceID, slog.String("rule", sniffResult.MatchedRule))
				ps.recordFailure(routeResult, FailureTypeSoft, FailureScopeKey, "假 200 响应")
				ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, routeResult.Channel.Name, routeResult.ModelConfigID, routeResult.Key.ID, errors.New("fake 200 response"), FailureTypeSoft)

				detailLog.EndTime(time.Now()).EndTime(time.Now()).Duration(int(time.Now().Sub(attemptStartTime).Milliseconds())).IsSuccess(false).Status("failed").ResponseBody(string(sniffResult.Body))
				mainLog.AddDetail(detailLog)
				mainLog.EndTime(time.Now()).Duration(int(time.Now().Sub(startTime))).StatusCode(http.StatusBadGateway).ErrorMessage("Fake 200 response").LastChannelID(routeResult.Channel.ID).LastChannelName(routeResult.Channel.Name).LastModel(routeResult.UpstreamModel)
				if !ps.retryCoordinator.ShouldRetry(retryCtx) {
					ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, "All upstream attempts failed", traceID)
					ps.auditLogger.LogAsync(mainLog.Build())
					return errors.New("fake 200 response")
				}

				_ = ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
				continue
			}

			if ep, epErr := endpoint.Get(endpointType); epErr == nil {
				if valid, errMsg := ep.ValidateResponse(upstreamResp.StatusCode, sniffResult.Body); !valid {
					ps.logErrorWithTrace("响应校验失败", traceID,
						slog.String("error", errMsg),
						slog.String("endpoint_type", endpointType),
					)
					ps.recordFailure(routeResult, FailureTypeSoft, FailureScopeModelConfig, errMsg)
					ps.retryCoordinator.RecordAttempt(retryCtx, routeResult.Channel.ID, routeResult.Channel.Name, routeResult.ModelConfigID, routeResult.Key.ID, errors.New(errMsg), FailureTypeSoft)
					detailLog.EndTime(time.Now()).EndTime(time.Now()).Duration(int(time.Now().Sub(attemptStartTime).Milliseconds())).IsSuccess(false).Status("failed").ResponseBody(string(sniffResult.Body)).ErrorMessage(errMsg)
					mainLog.AddDetail(detailLog)
					mainLog.EndTime(time.Now()).Duration(int(time.Now().Sub(startTime))).StatusCode(http.StatusBadGateway).ErrorMessage(errMsg).LastChannelID(routeResult.Channel.ID).LastChannelName(routeResult.Channel.Name).LastModel(routeResult.UpstreamModel)
					if !ps.retryCoordinator.ShouldRetry(retryCtx) {
						ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, "All upstream attempts failed", traceID)
						ps.auditLogger.LogAsync(mainLog.Build())
						return errors.New(errMsg)
					}

					_ = ps.retryCoordinator.WaitBeforeRetry(ctx, retryCtx)
					continue
				}
			} else {
				ps.logWarnWithTrace("未找到端点配置，跳过响应校验", traceID, slog.String("endpoint_type", endpointType))
			}

			responseBodyStr, forwardErr = ps.responseForwarder.ForwardJSONResponse(c, upstreamResp, routeResult.UpstreamModel, routeResult.UnifiedModel, traceID)
		}

		// 更新明细日志
		errorMsg := ""
		if forwardErr != nil {
			errorMsg = forwardErr.Error()
			ps.logErrorWithTrace("转发异常", traceID,
				slog.Uint64("channel_id", uint64(routeResult.Channel.ID)),
				slog.String("channel_name", routeResult.Channel.Name),
				slog.String("error", errorMsg),
			)
		}

		detailLog.ResponseBody(responseBodyStr).StreamChunks(streamChunks).StreamFirstChunkTime(firstChunkTime).
			IsSuccess(forwardErr == nil).
			Status("failed").
			ErrorMessage(errorMsg).EndTime(time.Now()).Duration(int(time.Now().Sub(attemptStartTime).Milliseconds()))

		if forwardErr == nil {
			promptTokens, completionTokens := ps.parseTokenUsage(endpointType, bodyBytes, responseBodyStr, isStream, traceID)
			detailLog.PromptTokens(promptTokens).CompletionTokens(completionTokens)
			mainLog.PromptTokens(promptTokens).CompletionTokens(completionTokens)
			ps.recordTokenUsageAsync(routeResult, getAccessTokenIDFromContext(c), promptTokens, completionTokens, traceID)
		}

		mainLog.AddDetail(detailLog)
		statusCode := upstreamResp.StatusCode
		mainLog.IsSuccess(forwardErr == nil).EndTime(time.Now()).Duration(int(time.Now().Sub(startTime))).StatusCode(statusCode).ErrorMessage(errorMsg).LastChannelID(routeResult.Channel.ID).LastChannelName(routeResult.Channel.Name).LastModel(routeResult.UpstreamModel)
		ps.auditLogger.LogAsync(mainLog.Build())
		return forwardErr
	}
}

// recordFailure 记录故障到熔断器
func (ps *ProxyService) recordFailure(routeResult *RouteResult, failureType FailureType, failureScope FailureScope, errMsg string) {
	switch failureScope {
	case FailureScopeKey:
		if failureType == FailureTypeHard {
			ps.circuitManager.RecordKeyHardFailure(routeResult.Key.ID, routeResult.Channel.ID, routeResult.Channel.Name, errMsg)
		} else if failureType == FailureTypeSoft {
			ps.circuitManager.RecordKeySoftFailure(routeResult.Key.ID, routeResult.Channel.ID, routeResult.Channel.Name, errMsg)
		}
	case FailureScopeModelConfig:
		if failureType == FailureTypeSoft || failureType == FailureTypeModelNotFound {
			ps.circuitManager.RecordModelConfigFailure(
				routeResult.ModelConfigID,
				routeResult.Channel.ID,
				routeResult.Channel.Name,
				routeResult.UnifiedModel,
				routeResult.UpstreamModel,
				errMsg,
			)
		}
	case FailureScopeBoth:
		if failureType == FailureTypeHard {
			ps.circuitManager.RecordKeyHardFailure(routeResult.Key.ID, routeResult.Channel.ID, routeResult.Channel.Name, errMsg)
		} else if failureType == FailureTypeSoft {
			ps.circuitManager.RecordKeySoftFailure(routeResult.Key.ID, routeResult.Channel.ID, routeResult.Channel.Name, errMsg)
		}
		if failureType == FailureTypeSoft || failureType == FailureTypeModelNotFound {
			ps.circuitManager.RecordModelConfigFailure(
				routeResult.ModelConfigID,
				routeResult.Channel.ID,
				routeResult.Channel.Name,
				routeResult.UnifiedModel,
				routeResult.UpstreamModel,
				errMsg,
			)
		}
	default:
		return
	}
}

func (ps *ProxyService) recordSuccess(routeResult *RouteResult) {
	ps.circuitManager.RecordKeySuccess(routeResult.Key.ID, routeResult.Channel.ID)
	if routeResult.ModelConfigID != 0 {
		ps.circuitManager.RecordModelConfigSuccess(routeResult.ModelConfigID, routeResult.Channel.ID)
	}
}

func (ps *ProxyService) parseTokenUsage(endpointType string, requestBody []byte, responseBody string, isStream bool, traceID string) (int64, int64) {
	ep, err := endpoint.Get(endpointType)
	if err != nil {
		ps.logWarnWithTrace("解析 token 失败，端点类型不存在", traceID, slog.String("endpoint_type", endpointType))
		return 0, 0
	}

	return ep.ParseTokenUsage(requestBody, responseBody, isStream)
}

func (ps *ProxyService) recordTokenUsageAsync(routeResult *RouteResult, accessTokenID uint, promptTokens, completionTokens int64, traceID string) {
	if promptTokens == 0 && completionTokens == 0 {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if ps.modelConfigRepo != nil && routeResult.ModelConfigID != 0 {
			if err := ps.modelConfigRepo.IncrementTokenUsage(ctx, routeResult.ModelConfigID, promptTokens, completionTokens); err != nil {
				ps.logWarnWithTrace("更新模型配置 token 统计失败", traceID, slog.String("error", err.Error()))
			}
		}

		if ps.keyRepo != nil && routeResult.Key != nil {
			if err := ps.keyRepo.IncrementTokenUsage(ctx, routeResult.Key.ID, promptTokens, completionTokens); err != nil {
				ps.logWarnWithTrace("更新密钥 token 统计失败", traceID, slog.String("error", err.Error()))
			}
		}

		if ps.accessTokenRepo != nil && accessTokenID != 0 {
			if err := ps.accessTokenRepo.IncrementTokenUsage(ctx, accessTokenID, promptTokens, completionTokens); err != nil {
				ps.logWarnWithTrace("更新访问令牌 token 统计失败", traceID, slog.String("error", err.Error()))
			}
		}
	}()
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

func getAccessTokenIDFromContext(c *gin.Context) uint {
	if tokenID, exists := c.Get("access_token_id"); exists {
		if id, ok := tokenID.(uint); ok {
			return id
		}
	}
	return 0
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

func readAndResetResponseBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func formatHeaders(headers http.Header) string {
	if len(headers) == 0 {
		return "<empty>"
	}

	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.Join(headers[key], ", ")
		lines = append(lines, fmt.Sprintf("- %s: %s", key, value))
	}

	return strings.Join(lines, "\n")
}

func formatBody(body []byte) string {
	if len(body) == 0 {
		return "<empty>"
	}

	trimmed := bytes.TrimSpace(body)
	if json.Valid(trimmed) {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, trimmed, "  ", "  "); err == nil {
			return pretty.String()
		}
	}

	if !utf8.Valid(body) {
		return fmt.Sprintf("<%d bytes; non-utf8>", len(body))
	}

	const maxBodyBytes = 4096
	text := string(body)
	if len(text) > maxBodyBytes {
		return text[:maxBodyBytes] + fmt.Sprintf("\n<... truncated, %d bytes total>", len(body))
	}

	return text
}

func indentBlock(text, indent string) string {
	if text == "" {
		return indent + "<empty>"
	}

	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}

// OnConfigChanged 实现 ConfigListener 接口，响应配置变更
func (ps *ProxyService) OnConfigChanged(ctx context.Context, category string) {
	// 处理 proxy 分类的配置变更
	if category == "proxy" {
		requestTimeout, _, maxRetry := ps.settingService.GetProxyConfig(ctx)
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
func (ps *ProxyService) getEndpointType(endpointPath string) (string, error) {
	// 从端点注册中心查找匹配的端点
	for _, ep := range endpoint.GetAll() {
		epPath := ep.GetPath()
		if epPath == endpointPath {
			return ep.GetType(), nil
		}
		if strings.Contains(epPath, "*") {
			prefix := strings.Split(epPath, "*")[0]
			if prefix != "" && strings.HasPrefix(endpointPath, prefix) {
				return ep.GetType(), nil
			}
		}
	}

	return "openai", errors.New("unsupported endpoint")
}
