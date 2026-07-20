package proxy

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/models"
)

// =============================================================================
// Trace 日志
// =============================================================================

func (ps *ProxyService) logTraceInfo(traceID, msg string, attrs ...slog.Attr) {
	ps.emitTrace(slog.LevelInfo, traceID, msg, attrs)
}

func (ps *ProxyService) logTraceDebug(traceID, msg string, attrs ...slog.Attr) {
	ps.emitTrace(slog.LevelDebug, traceID, msg, attrs)
}

// emitTrace 仅在调试模式启用时输出；trace_id 始终作为首字段
func (ps *ProxyService) emitTrace(level slog.Level, traceID, msg string, attrs []slog.Attr) {
	if !ps.isDebugModeEnabled() {
		return
	}
	allAttrs := make([]any, 0, len(attrs)+1)
	allAttrs = append(allAttrs, slog.String("trace_id", normalizeProxyTraceID(traceID)))
	for _, attr := range attrs {
		allAttrs = append(allAttrs, attr)
	}
	if level == slog.LevelDebug {
		ps.logger.Debug(msg, allAttrs...)
		return
	}
	ps.logger.Info(msg, allAttrs...)
}

// =============================================================================
// 请求汇总
// =============================================================================

// requestSummary 本次请求的汇总状态
type requestSummary struct {
	traceID         string
	statusCode      int
	clientCancelled bool
	duration        time.Duration
	routeAttempts   int
	retryCount      int
	model           string
	channelDisplay  string
	channelName     string
	channelModel    string
	keyMasked       string
	endpointType    string
	channelID       uint
	keyID           uint
	modelConfigID   uint
}

func buildRequestSummary(c *gin.Context, proxyCtx *ProxyContext, err error) requestSummary {
	clientCancelled := proxyCtx.LastFailureStage == stageClientCancelled

	routeAttempts := proxyCtx.RouteAttempts
	if routeAttempts < proxyCtx.AttemptCount {
		routeAttempts = proxyCtx.AttemptCount
	}
	retryCount := 0
	if routeAttempts > 0 {
		retryCount = routeAttempts - 1
	}

	model := proxyCtx.Model
	if strings.TrimSpace(model) == "" {
		model = "-"
	}

	s := requestSummary{
		traceID:         normalizeProxyTraceID(proxyCtx.TraceID),
		statusCode:      resolveFinalStatusCode(c.Writer.Status(), err, clientCancelled, proxyCtx.ResponseStatusLocked),
		clientCancelled: clientCancelled,
		duration:        time.Since(proxyCtx.StartTime),
		routeAttempts:   routeAttempts,
		retryCount:      retryCount,
		model:           model,
		channelName:     "-",
		channelModel:    "-",
		keyMasked:       "-",
		endpointType:    "-",
	}
	if proxyCtx.Endpoint != nil {
		s.endpointType = proxyCtx.Endpoint.GetType()
	}
	if proxyCtx.LastRoute != nil {
		s.channelID = proxyCtx.LastRoute.ChannelID
		s.channelName = proxyCtx.LastRoute.ChannelName
		s.keyID = proxyCtx.LastRoute.KeyID
		s.keyMasked = proxyCtx.LastRoute.KeyMasked
		s.modelConfigID = proxyCtx.LastRoute.ModelConfigID
		s.channelModel = proxyCtx.LastRoute.ChannelModel
		if proxyCtx.LastRoute.EndpointType != "" {
			s.endpointType = proxyCtx.LastRoute.EndpointType
		}
	}

	s.channelDisplay = s.channelName
	if s.channelDisplay == "" || s.channelDisplay == "-" {
		s.channelDisplay = "-"
	} else if s.channelID != 0 {
		s.channelDisplay = fmt.Sprintf("%s#%d", s.channelName, s.channelID)
	}

	return s
}

// resolveFinalStatusCode 按"写入状态优先、client_cancelled 次之、err 次之"的顺序解析最终日志状态码。
// 当响应状态码已被非流式保活提前锁定时，日志应保留真实写出的 HTTP 状态码。
func resolveFinalStatusCode(written int, err error, clientCancelled bool, statusLocked bool) int {
	if written != 0 {
		if statusLocked {
			return written
		}
		if err != nil && !clientCancelled && written < http.StatusBadRequest {
			return http.StatusBadGateway
		}
		return written
	}
	switch {
	case clientCancelled:
		return clientCancelledStatusCode
	case err != nil:
		return http.StatusBadGateway
	default:
		return http.StatusOK
	}
}

func formatRequestSummaryMessage(c *gin.Context, s requestSummary) string {
	return fmt.Sprintf(
		"代理完成 | %d %s %s | 耗时=%s | 模型=%s | 渠道=%s | 渠道模型=%s | 密钥=%s | 端点=%s | 重试=%d",
		s.statusCode,
		c.Request.Method,
		c.Request.URL.Path,
		s.duration.Truncate(time.Millisecond),
		s.model,
		s.channelDisplay,
		s.channelModel,
		s.keyMasked,
		s.endpointType,
		s.retryCount,
	)
}

func buildRequestSummaryAttrs(c *gin.Context, proxyCtx *ProxyContext, err error, s requestSummary) []any {
	attrs := []any{
		slog.String("component", "proxy"),
		slog.String("event", "proxy.request.completed"),
		slog.String("trace_id", s.traceID),
		slog.Int("status_code", s.statusCode),
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.Duration("duration", s.duration),
		slog.String("model", s.model),
		slog.String("endpoint_type", s.endpointType),
		slog.Bool("request_stream", proxyCtx.IsStreamRequest),
		slog.Int("route_attempts", s.routeAttempts),
		slog.Int("retry_count", s.retryCount),
		slog.Uint64("channel_id", uint64(s.channelID)),
		slog.String("channel_name", s.channelName),
		slog.String("channel_model", s.channelModel),
		slog.Uint64("model_config_id", uint64(s.modelConfigID)),
		slog.Uint64("key_id", uint64(s.keyID)),
		slog.String("key_masked", s.keyMasked),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		attrs = append(attrs,
			slog.String("final_error_code", resolveFinalErrorCode(proxyCtx, s)),
			slog.String("final_error_message", truncateErrorMessage(err.Error())),
		)
	} else if s.clientCancelled {
		attrs = append(attrs, slog.String("final_error_code", stageClientCancelled))
	}
	if proxyCtx.LastFailureType != FailureTypeNone {
		attrs = append(attrs, slog.String("last_failure_type", string(proxyCtx.LastFailureType)))
	}
	if proxyCtx.LastFailureScope != FailureScopeNone {
		attrs = append(attrs, slog.String("last_failure_scope", string(proxyCtx.LastFailureScope)))
	}
	if proxyCtx.LastFailureStage != "" {
		attrs = append(attrs, slog.String("last_failure_stage", proxyCtx.LastFailureStage))
	}
	if attempt := proxyCtx.CurrentAttempt(); attempt != nil {
		attrs = append(attrs,
			slog.Int("last_attempt_num", attempt.AttemptNum),
			slog.Int("last_attempt_status_code", attempt.UpstreamStatusCode),
			slog.Uint64("last_attempt_channel_id", uint64(attempt.ChannelID)),
			slog.String("last_attempt_channel_name", attempt.ChannelName),
			slog.Uint64("last_attempt_model_config_id", uint64(attempt.ModelConfigID)),
			slog.Uint64("last_attempt_key_id", uint64(attempt.KeyID)),
		)
		if attempt.FailureType != FailureTypeNone {
			attrs = append(attrs, slog.String("last_attempt_failure_type", string(attempt.FailureType)))
		}
		if attempt.FailureScope != FailureScopeNone {
			attrs = append(attrs, slog.String("last_attempt_failure_scope", string(attempt.FailureScope)))
		}
		if attempt.FailureStage != "" {
			attrs = append(attrs, slog.String("last_attempt_failure_stage", attempt.FailureStage))
		}
		if attempt.ErrorMessage != "" {
			attrs = append(attrs, slog.String("last_attempt_error", attempt.ErrorMessage))
		}
	}
	return attrs
}

func resolveFinalErrorCode(proxyCtx *ProxyContext, s requestSummary) string {
	if s.clientCancelled {
		return stageClientCancelled
	}
	if proxyCtx == nil {
		return "proxy_error"
	}
	if proxyCtx.LastFailureStage != "" {
		return proxyCtx.LastFailureStage
	}
	if proxyCtx.LastFailureType != FailureTypeNone && proxyCtx.LastFailureScope != FailureScopeNone {
		return string(proxyCtx.LastFailureType) + "_" + string(proxyCtx.LastFailureScope)
	}
	if proxyCtx.LastFailureType != FailureTypeNone {
		return string(proxyCtx.LastFailureType)
	}
	return "proxy_error"
}

func (ps *ProxyService) emitRequestSummary(s requestSummary, msg string, attrs []any) {
	switch {
	case s.clientCancelled:
		ps.logger.Info(msg, attrs...)
	case s.statusCode >= http.StatusInternalServerError:
		ps.logger.Error(msg, attrs...)
	case s.statusCode >= http.StatusBadRequest:
		ps.logger.Warn(msg, attrs...)
	default:
		ps.logger.Info(msg, attrs...)
	}
}

// logFinalRequestResult 在 ProxyRequest 退出时输出一条请求汇总日志
func (ps *ProxyService) logFinalRequestResult(c *gin.Context, proxyCtx *ProxyContext, err error) {
	if proxyCtx != nil && proxyCtx.SuppressLogging {
		return
	}
	summary := buildRequestSummary(c, proxyCtx, err)
	msg := formatRequestSummaryMessage(c, summary)
	attrs := buildRequestSummaryAttrs(c, proxyCtx, err, summary)
	ps.emitRequestSummary(summary, msg, attrs)
	ps.persistRequestLog(c, proxyCtx, err, summary)
}

// persistRequestLog 把本次请求投递到 RequestLogRecorder（异步落表）
// 主表与上游尝试基础信息始终写；客户端/上游请求响应报文与头仅在调试模式开启时写。
func (ps *ProxyService) persistRequestLog(c *gin.Context, proxyCtx *ProxyContext, retErr error, summary requestSummary) {
	if ps.requestLogRecorder == nil {
		return
	}
	if proxyCtx != nil && proxyCtx.SuppressLogging {
		return
	}

	log := &models.RequestLog{
		CreatedAt:          time.Now(),
		TraceID:            normalizeProxyTraceID(proxyCtx.TraceID),
		ClientIP:           c.ClientIP(),
		AccessTokenID:      getAccessTokenIDFromContext(c),
		AccessTokenName:    getAccessTokenNameFromContext(c),
		Method:             c.Request.Method,
		Path:               c.Request.URL.Path,
		EndpointType:       summary.endpointType,
		Model:              proxyCtx.Model,
		IsStream:           proxyCtx.IsStreamRequest,
		StatusCode:         summary.statusCode,
		Success:            retErr == nil && !summary.clientCancelled,
		DurationMS:         summary.duration.Milliseconds(),
		RouteAttempts:      summary.routeAttempts,
		RetryCount:         summary.retryCount,
		FinalChannelID:     summary.channelID,
		FinalChannelName:   summary.channelName,
		FinalKeyID:         summary.keyID,
		FinalModelConfigID: summary.modelConfigID,
		FinalChannelModel:  summary.channelModel,
		PromptTokens:       proxyCtx.PromptTokens,
		CompletionTokens:   proxyCtx.CompletionTokens,
		FailureType:        string(proxyCtx.LastFailureType),
		FailureScope:       string(proxyCtx.LastFailureScope),
		FailureStage:       proxyCtx.LastFailureStage,
	}
	if retErr != nil {
		log.ErrorMessage = truncateErrorMessage(retErr.Error())
	}
	// FailureTypeNone / FailureScopeNone 持久化为空字符串更简洁
	if log.FailureType == string(FailureTypeNone) {
		log.FailureType = ""
	}
	if log.FailureScope == string(FailureScopeNone) {
		log.FailureScope = ""
	}

	debugMode := ps.isDebugModeEnabled()
	event := RequestLogEvent{
		Log:      log,
		Attempts: buildRequestLogAttempts(log, proxyCtx.Attempts, debugMode),
	}

	if debugMode {
		event.Detail = &models.RequestLogDetail{
			TraceID:             log.TraceID,
			CreatedAt:           log.CreatedAt,
			RequestHeadersJSON:  marshalHeaders(sanitizeHeaders(proxyCtx.RequestHeaders)),
			RequestBody:         string(proxyCtx.RequestBody),
			RequestBodySize:     int64(len(proxyCtx.RequestBody)),
			ResponseHeadersJSON: marshalHeaders(sanitizeHeaders(c.Writer.Header())),
			ResponseBody:        string(proxyCtx.ResponseBody),
			ResponseBodySize:    int64(len(proxyCtx.ResponseBody)),
		}
	}

	ps.requestLogRecorder.Record(event)
}

func buildRequestLogAttempts(log *models.RequestLog, attempts []*AttemptRecord, includePayload bool) []*models.RequestLogAttempt {
	if log == nil || len(attempts) == 0 {
		return nil
	}
	result := make([]*models.RequestLogAttempt, 0, len(attempts))
	for _, a := range attempts {
		if a == nil {
			continue
		}
		item := &models.RequestLogAttempt{
			CreatedAt:          log.CreatedAt,
			TraceID:            log.TraceID,
			AttemptNum:         a.AttemptNum,
			ChannelID:          a.ChannelID,
			ChannelName:        a.ChannelName,
			ModelConfigID:      a.ModelConfigID,
			Model:              a.Model,
			ChannelModel:       a.ChannelModel,
			KeyID:              a.KeyID,
			KeyName:            a.KeyName,
			KeyMasked:          a.KeyMasked,
			UpstreamURL:        a.UpstreamURL,
			DurationMS:         a.DurationMS,
			UpstreamStatusCode: a.UpstreamStatusCode,
			Success:            a.Success,
			FailureType:        failureTypeString(a.FailureType),
			FailureScope:       failureScopeString(a.FailureScope),
			FailureStage:       a.FailureStage,
			ErrorMessage:       a.ErrorMessage,
		}
		if includePayload {
			item.UpstreamRequestHeadersJSON = marshalHeaders(sanitizeHeaders(a.UpstreamRequestHeaders))
			item.UpstreamRequestBody = string(a.UpstreamRequestBody)
			item.UpstreamRequestBodySize = int64(len(a.UpstreamRequestBody))
			item.UpstreamResponseHeadersJSON = marshalHeaders(sanitizeHeaders(a.UpstreamResponseHeaders))
			item.UpstreamResponseBody = string(a.UpstreamResponseBody)
			item.UpstreamResponseBodySize = int64(len(a.UpstreamResponseBody))
		}
		result = append(result, item)
	}
	return result
}
