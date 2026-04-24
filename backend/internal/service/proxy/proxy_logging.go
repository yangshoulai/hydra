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
		statusCode:      resolveFinalStatusCode(c.Writer.Status(), err, clientCancelled),
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

// resolveFinalStatusCode 按"写入状态优先、client_cancelled 次之、err 次之"的顺序解析最终日志状态码
func resolveFinalStatusCode(written int, err error, clientCancelled bool) int {
	if written != 0 {
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
	return attrs
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
// 主表始终写；详情 / 尝试明细仅在调试模式开启时写。
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

	event := RequestLogEvent{Log: log}

	if ps.isDebugModeEnabled() {
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

		attempts := make([]*models.RequestLogAttempt, 0, len(proxyCtx.Attempts))
		for _, a := range proxyCtx.Attempts {
			attempts = append(attempts, &models.RequestLogAttempt{
				CreatedAt:                   log.CreatedAt,
				TraceID:                     log.TraceID,
				AttemptNum:                  a.AttemptNum,
				ChannelID:                   a.ChannelID,
				ChannelName:                 a.ChannelName,
				ChannelModel:                a.ChannelModel,
				KeyID:                       a.KeyID,
				KeyName:                     a.KeyName,
				KeyMasked:                   a.KeyMasked,
				UpstreamURL:                 a.UpstreamURL,
				DurationMS:                  a.DurationMS,
				UpstreamStatusCode:          a.UpstreamStatusCode,
				Success:                     a.Success,
				FailureType:                 failureTypeString(a.FailureType),
				FailureScope:                failureScopeString(a.FailureScope),
				FailureStage:                a.FailureStage,
				ErrorMessage:                a.ErrorMessage,
				UpstreamRequestHeadersJSON:  marshalHeaders(sanitizeHeaders(a.UpstreamRequestHeaders)),
				UpstreamRequestBody:         string(a.UpstreamRequestBody),
				UpstreamRequestBodySize:     int64(len(a.UpstreamRequestBody)),
				UpstreamResponseHeadersJSON: marshalHeaders(sanitizeHeaders(a.UpstreamResponseHeaders)),
				UpstreamResponseBody:        string(a.UpstreamResponseBody),
				UpstreamResponseBodySize:    int64(len(a.UpstreamResponseBody)),
			})
		}
		event.Attempts = attempts
	}

	ps.requestLogRecorder.Record(event)
}
