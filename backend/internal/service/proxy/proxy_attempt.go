package proxy

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/endpoint"
	"github.com/yangshoulai/hydra/internal/service/metrics"
)

// =============================================================================
// 阶段 1：请求预处理
// =============================================================================

// prepareRequest 读请求体、解析模型、校验权限与可用性；
// 任一步失败都已完成错误响应写出与失败指标记录。
func (ps *ProxyService) prepareRequest(c *gin.Context, ep endpoint.Endpoint, proxyCtx *ProxyContext) error {
	traceID := proxyCtx.TraceID

	// 快照客户端请求头（脱敏前原文），用于调试模式下的请求日志详情
	proxyCtx.RequestHeaders = c.Request.Header.Clone()

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		ps.logTraceDebug(traceID, "读取请求体失败", slog.String("error", err.Error()))
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadRequest, ErrReadRequestBodyFailed.Error(), traceID)
		ps.recordRequestMetrics(false, "", nil, 0, 0)
		return ErrReadRequestBodyFailed
	}
	proxyCtx.RequestBody = body
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	model, err := ep.GetModelFromRequest(c.Request, body)
	if err != nil {
		ps.logTraceDebug(traceID, "解析模型失败", slog.String("error", err.Error()))
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadRequest, err.Error(), traceID)
		ps.recordRequestMetrics(false, "", nil, 0, 0)
		return err
	}
	proxyCtx.Model = model

	if ok, reason := isModelAllowedForToken(c, model); !ok {
		ps.logTraceDebug(traceID, "令牌模型权限校验失败",
			slog.String("model", model),
			slog.String("reason", reason),
		)
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusForbidden, reason, traceID)
		ps.recordRequestMetrics(false, model, nil, 0, 0)
		return ErrModelNotAllowedForToken
	}

	proxyCtx.IsStreamRequest = ps.requestBuilder.IsStreamRequest(body, ep.GetType(), c.Request.URL.Path)

	supported, err := ps.modelConfigRepo.ExistsActiveModel(c.Request.Context(), model, ep.GetType())
	if err != nil {
		ps.logTraceDebug(traceID, "模型可用性校验失败",
			slog.String("model", model),
			slog.String("error", err.Error()),
		)
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusServiceUnavailable, "model unavailable", traceID)
		ps.recordRequestMetrics(false, model, nil, 0, 0)
		return err
	}
	if !supported {
		proxyCtx.SuppressLogging = true
		MarkProxyLoggingSuppressed(c)
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusNotFound, "model not found: "+model, traceID)
		ps.recordRequestMetrics(false, model, nil, 0, 0)
		return ErrModelNotFound
	}

	return nil
}

// =============================================================================
// 阶段 3：单次尝试
// =============================================================================

// attemptOnce 执行一次路由结果下的上游调用与响应处理。
// 返回 (shouldRetry, err)：shouldRetry 为 true 时外层应继续下一轮循环。
func (ps *ProxyService) attemptOnce(c *gin.Context, proxyCtx *ProxyContext, routeResult *RouteResult) (bool, error) {
	proxyCtx.AttemptStartTime = time.Now()
	proxyCtx.RouteAttempts++
	proxyCtx.LastRoute = newRouteSnapshot(routeResult, proxyCtx.Endpoint.GetType())

	attempt := &AttemptRecord{
		AttemptNum:    proxyCtx.RouteAttempts,
		ChannelID:     routeResult.Channel.ID,
		ChannelName:   routeResult.Channel.Name,
		ChannelModel:  routeResult.ChannelModel,
		KeyID:         routeResult.Key.ID,
		KeyName:       routeResult.Key.Remark,
		KeyMasked:     proxyCtx.LastRoute.KeyMasked,
		ModelConfigID: routeResult.ModelConfigID,
		StartedAt:     proxyCtx.AttemptStartTime,
	}
	proxyCtx.Attempts = append(proxyCtx.Attempts, attempt)

	ps.logTraceInfo(proxyCtx.TraceID, "路由成功",
		slog.Int("attempt", proxyCtx.RouteAttempts),
		slog.Uint64("channel_id", uint64(routeResult.Channel.ID)),
		slog.String("channel_name", routeResult.Channel.Name),
		slog.Uint64("key_id", uint64(routeResult.Key.ID)),
		slog.String("key_masked", proxyCtx.LastRoute.KeyMasked),
		slog.Uint64("model_config_id", uint64(routeResult.ModelConfigID)),
		slog.Int("model_weight", routeResult.ModelWeight),
		slog.String("channel_model", routeResult.ChannelModel),
	)

	upstreamReq, _, buildErr := ps.requestBuilder.BuildProxyRequest(c, routeResult, proxyCtx.Endpoint)
	if buildErr != nil {
		ps.finalizeAttempt(attempt, nil, false, buildErr.Error())
		return ps.retryOrFail(c, proxyCtx, routeResult, buildErr, FailureTypeHard, FailureScopeNone, "构建上游请求失败")
	}

	// 采集上游请求信息（URL 和 header 始终采集；body 仅调试模式保留）
	attempt.UpstreamURL = upstreamReq.URL.String()
	attempt.UpstreamRequestHeaders = upstreamReq.Header.Clone()
	if ps.isDebugModeEnabled() {
		if captured, replacedReq, err := snapshotRequestBody(upstreamReq); err == nil {
			attempt.UpstreamRequestBody = captured
			upstreamReq = replacedReq
		}
	}

	upstreamResp, requestErr := ps.httpClient.DoWithProxy(upstreamReq, proxyCtx.TraceID, routeResult.Channel.UseProxy)

	// 客户端主动断开：不记熔断、不重试、不计失败指标
	if isClientCancelled(c, requestErr) {
		if upstreamResp != nil {
			ps.captureUpstreamResponse(attempt, upstreamResp, false)
		}
		drainAndCloseBody(upstreamResp)
		ps.finalizeAttempt(attempt, upstreamResp, false, "client cancelled")
		return false, ps.markClientCancelled(c, proxyCtx, requestErr)
	}

	failureType, failureScope, errMsg := ps.failureClassifier.ClassifyResponseError(upstreamResp, requestErr)
	if failureType != FailureTypeNone {
		// 失败分支：调试模式下读完 body 再走 drain；非调试直接 drain
		ps.captureUpstreamResponse(attempt, upstreamResp, ps.isDebugModeEnabled())
		drainAndCloseBody(upstreamResp)
		attempt.FailureType = failureType
		attempt.FailureScope = failureScope
		ps.finalizeAttempt(attempt, upstreamResp, false, errMsg)
		return ps.retryOrFail(c, proxyCtx, routeResult, NewUpstreamCallError(errMsg), failureType, failureScope, "上游调用失败")
	}

	// 成功拿到上游响应：先快照头和状态码，body 在 handle* 内采集
	if upstreamResp != nil {
		attempt.UpstreamResponseHeaders = upstreamResp.Header.Clone()
		attempt.UpstreamStatusCode = upstreamResp.StatusCode
	}

	upstreamIsStream := proxyCtx.IsStreamRequest && isEventStreamResponse(upstreamResp)
	if proxyCtx.IsStreamRequest && !upstreamIsStream {
		ps.logTraceInfo(proxyCtx.TraceID, "请求为流式但上游返回非流式，降级为普通响应",
			slog.String("content_type", upstreamResp.Header.Get("Content-Type")),
		)
	}

	var handleErr error
	if upstreamIsStream {
		handleErr = ps.handleStreamUpstream(c, proxyCtx, routeResult, upstreamResp)
	} else {
		handleErr = ps.handleNonStreamUpstream(c, proxyCtx, routeResult, upstreamResp)
	}
	return ps.decideAfterHandle(c, proxyCtx, routeResult, handleErr)
}

// finalizeAttempt 补齐 AttemptRecord 结束字段
func (ps *ProxyService) finalizeAttempt(attempt *AttemptRecord, resp *http.Response, success bool, errMsg string) {
	if attempt == nil {
		return
	}
	attempt.DurationMS = time.Since(attempt.StartedAt).Milliseconds()
	if resp != nil && attempt.UpstreamStatusCode == 0 {
		attempt.UpstreamStatusCode = resp.StatusCode
	}
	attempt.Success = success
	if errMsg != "" {
		attempt.ErrorMessage = truncateErrorMessage(errMsg)
	}
}

// captureUpstreamResponse 采集失败/客户端取消分支的上游响应信息
// readBody=true 时读完整 body（供调试模式持久化）；readBody=false 时只快照头
func (ps *ProxyService) captureUpstreamResponse(attempt *AttemptRecord, resp *http.Response, readBody bool) {
	if attempt == nil || resp == nil {
		return
	}
	attempt.UpstreamResponseHeaders = resp.Header.Clone()
	attempt.UpstreamStatusCode = resp.StatusCode
	if !readBody {
		return
	}
	// 读完后 body 已消耗，后续 drainAndCloseBody 无副作用
	attempt.UpstreamResponseBody = readAndCloseBody(resp)
}

// retryOrFail 封装 tryScheduleRetry 的两种返回，供 attemptOnce 简洁上抛
func (ps *ProxyService) retryOrFail(
	c *gin.Context, proxyCtx *ProxyContext, routeResult *RouteResult,
	err error, failureType FailureType, failureScope FailureScope, stage string,
) (bool, error) {
	if ps.tryScheduleRetry(c, proxyCtx, routeResult, err, failureType, failureScope, stage) {
		return true, nil
	}
	return false, err
}

// decideAfterHandle 根据转发阶段返回的错误决定后续：成功结束 / 终止 / 进入下一轮重试
func (ps *ProxyService) decideAfterHandle(
	c *gin.Context, proxyCtx *ProxyContext, routeResult *RouteResult, handleErr error,
) (bool, error) {
	if handleErr == nil {
		return false, nil
	}
	retryErr, ok := AsRetryableProxyError(handleErr)
	if !ok {
		proxyCtx.LastError = handleErr
		proxyCtx.LastFailureStage = stageHandleResponse
		return false, handleErr
	}
	return ps.retryOrFail(c, proxyCtx, routeResult, retryErr.Cause, retryErr.FailureType, retryErr.FailureScope, retryErr.Stage)
}

// =============================================================================
// 阶段 4：响应处理（流式 / 非流式）
// =============================================================================

// handleStreamUpstream 处理流式上游响应：嗅探 → 转发 → 成功统计
// 返回 nil 表示成功；RetryableProxyError 由外层循环决定重试；普通 error 终止。
func (ps *ProxyService) handleStreamUpstream(
	c *gin.Context,
	proxyCtx *ProxyContext,
	routeResult *RouteResult,
	upstreamResp *http.Response,
) error {
	attempt := proxyCtx.CurrentAttempt()

	// 未进入 forwarder 的错误路径统一在此兜底关闭 upstream 连接
	forwardStarted := false
	defer func() {
		if !forwardStarted {
			drainAndCloseBody(upstreamResp)
		}
	}()

	snifferEnabled, streamPacketCount := ps.getSnifferConfig()
	probeFirstChunkMS := 0
	if snifferEnabled {
		probePayload, firstChunkMS, probeErr := ps.readStreamSniffPayload(upstreamResp, streamPacketCount, proxyCtx.TraceID)
		if emptyErr, ok := probeErr.(*EmptySSEBodyError); ok {
			ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeNone, stageStreamProbe, emptyErr.Error())
			return NewRetryableProxyError(emptyErr, FailureTypeSoft, FailureScopeNone, "空流式响应")
		}
		if probeErr != nil {
			if isClientCancelled(c, probeErr) {
				ps.finalizeAttempt(attempt, upstreamResp, false, "client cancelled")
				return ps.markClientCancelled(c, proxyCtx, probeErr)
			}
			ps.logTraceDebug(proxyCtx.TraceID, "流式嗅探预读失败", slog.String("error", probeErr.Error()))
			proxyCtx.LastError = probeErr
			proxyCtx.LastFailureStage = stageStreamProbe
			ps.finalizeAttempt(attempt, upstreamResp, false, probeErr.Error())
			ps.recordRequestMetrics(false, proxyCtx.Model, routeResult, 0, 0)
			return probeErr
		}
		probeFirstChunkMS = firstChunkMS

		sniffResult, sniffErr := ps.responseSniffer.Sniff(upstreamResp, true, probePayload, proxyCtx.TraceID)
		if sniffErr != nil {
			ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeNone, stageStreamProbe, sniffErr.Error())
			return NewRetryableProxyError(sniffErr, FailureTypeSoft, FailureScopeNone, "流式嗅探失败")
		}
		if sniffResult == nil {
			ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeNone, stageStreamProbe, ErrEmptySniffResult.Error())
			return NewRetryableProxyError(ErrEmptySniffResult, FailureTypeSoft, FailureScopeNone, "流式嗅探结果为空")
		}
		if sniffResult.IsFake200 {
			ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeKey, stageStreamProbe, ErrFake200Response.Error())
			return NewRetryableProxyError(ErrFake200Response, FailureTypeSoft, FailureScopeKey, "流式假200")
		}

		// 回灌预读数据：multiReadCloser 保留 Close 语义以便连接归还到池
		upstreamResp.Body = newMultiReadCloser(probePayload, upstreamResp.Body)
	}

	// 转发接管 body，其内部 defer Close 会触发 multiReadCloser.Close → 原始 body.Close
	forwardStarted = true
	forwardResult, forwardErr := ps.responseForwarder.ForwardStreamResponse(c, upstreamResp, proxyCtx.TraceID)
	if emptyErr, ok := forwardErr.(*EmptySSEBodyError); ok {
		ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeNone, stageStreamForward, emptyErr.Error())
		return NewRetryableProxyError(emptyErr, FailureTypeSoft, FailureScopeNone, "空流式响应")
	}
	if forwardErr != nil {
		if isClientCancelled(c, forwardErr) {
			ps.finalizeAttempt(attempt, upstreamResp, false, "client cancelled")
			return ps.markClientCancelled(c, proxyCtx, forwardErr)
		}
		ps.logTraceDebug(proxyCtx.TraceID, "流式转发失败", slog.String("error", forwardErr.Error()))
		proxyCtx.LastError = forwardErr
		proxyCtx.LastFailureStage = stageStreamForward
		ps.finalizeAttempt(attempt, upstreamResp, false, forwardErr.Error())
		ps.recordRequestMetrics(false, proxyCtx.Model, routeResult, 0, 0)
		return forwardErr
	}
	if forwardResult == nil {
		ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeNone, stageStreamForward, ErrEmptySniffResult.Error())
		return NewRetryableProxyError(ErrEmptySniffResult, FailureTypeSoft, FailureScopeNone, "流式转发结果为空")
	}

	// 成功：流式场景下上游 body 与客户端响应 body 相同
	if attempt != nil && ps.isDebugModeEnabled() {
		attempt.UpstreamResponseBody = []byte(forwardResult.ResponseBody)
	}
	if ps.isDebugModeEnabled() {
		proxyCtx.ResponseBody = []byte(forwardResult.ResponseBody)
	}
	ps.finalizeAttempt(attempt, upstreamResp, true, "")

	promptTokens, completionTokens := ps.recordAttemptSuccess(c, proxyCtx, routeResult, forwardResult.ResponseBody, true)

	firstChunkMS := forwardResult.FirstChunkMS
	if probeFirstChunkMS > 0 {
		firstChunkMS = probeFirstChunkMS
	}
	ps.logTraceInfo(proxyCtx.TraceID, "上游响应处理完成（流式）",
		slog.Duration("duration", time.Since(proxyCtx.StartTime)),
		slog.Int("stream_chunks", forwardResult.StreamChunks),
		slog.Int("first_chunk_ms", firstChunkMS),
		slog.Int64("prompt_tokens", promptTokens),
		slog.Int64("completion_tokens", completionTokens),
	)
	return nil
}

// handleNonStreamUpstream 处理非流式上游响应：读取 → 嗅探 → 校验 → 转发 → 成功统计
func (ps *ProxyService) handleNonStreamUpstream(
	c *gin.Context,
	proxyCtx *ProxyContext,
	routeResult *RouteResult,
	upstreamResp *http.Response,
) error {
	attempt := proxyCtx.CurrentAttempt()
	defer func() { _ = upstreamResp.Body.Close() }()

	body, readErr := io.ReadAll(upstreamResp.Body)
	if readErr != nil {
		if isClientCancelled(c, readErr) {
			ps.finalizeAttempt(attempt, upstreamResp, false, "client cancelled")
			return ps.markClientCancelled(c, proxyCtx, readErr)
		}
		ps.logTraceDebug(proxyCtx.TraceID, "读取非流式响应失败", slog.String("error", readErr.Error()))
		proxyCtx.LastError = readErr
		proxyCtx.LastFailureStage = stageNonStreamRead
		ps.finalizeAttempt(attempt, upstreamResp, false, readErr.Error())
		ps.recordRequestMetrics(false, proxyCtx.Model, routeResult, 0, 0)
		return readErr
	}

	// body 拿到即为原始上游响应，attach 到 attempt（仅调试模式保留）
	if attempt != nil && ps.isDebugModeEnabled() {
		attempt.UpstreamResponseBody = append([]byte(nil), body...)
	}

	snifferEnabled, _ := ps.getSnifferConfig()
	if snifferEnabled {
		sniffResult, sniffErr := ps.responseSniffer.Sniff(upstreamResp, false, body, proxyCtx.TraceID)
		if sniffErr != nil {
			ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeNone, stageNonStreamRead, sniffErr.Error())
			return NewRetryableProxyError(sniffErr, FailureTypeSoft, FailureScopeNone, "非流式嗅探失败")
		}
		if sniffResult == nil {
			ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeNone, stageNonStreamRead, ErrEmptySniffResult.Error())
			return NewRetryableProxyError(ErrEmptySniffResult, FailureTypeSoft, FailureScopeNone, "非流式嗅探结果为空")
		}
		if sniffResult.IsFake200 {
			ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeKey, stageNonStreamRead, ErrFake200Response.Error())
			return NewRetryableProxyError(ErrFake200Response, FailureTypeSoft, FailureScopeKey, "非流式假200")
		}
	}

	if valid, validateMsg := proxyCtx.Endpoint.ValidateResponse(upstreamResp.StatusCode, body); !valid {
		ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeModelConfig, stageNonStreamRead, validateMsg)
		return NewRetryableProxyError(NewResponseValidationError(validateMsg), FailureTypeSoft, FailureScopeModelConfig, "响应校验失败")
	}

	forwardResult, forwardErr := ps.responseForwarder.ForwardNonStreamResponse(
		c, upstreamResp, body, routeResult.ChannelModel, routeResult.Model, proxyCtx.TraceID,
	)
	if forwardErr != nil {
		if isClientCancelled(c, forwardErr) {
			ps.finalizeAttempt(attempt, upstreamResp, false, "client cancelled")
			return ps.markClientCancelled(c, proxyCtx, forwardErr)
		}
		ps.logTraceDebug(proxyCtx.TraceID, "响应转发失败", slog.String("error", forwardErr.Error()))
		proxyCtx.LastError = forwardErr
		proxyCtx.LastFailureStage = stageNonStreamForward
		ps.finalizeAttempt(attempt, upstreamResp, false, forwardErr.Error())
		ps.recordRequestMetrics(false, proxyCtx.Model, routeResult, 0, 0)
		return forwardErr
	}
	if forwardResult == nil {
		ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeNone, stageNonStreamForward, ErrEmptySniffResult.Error())
		return NewRetryableProxyError(ErrEmptySniffResult, FailureTypeSoft, FailureScopeNone, "非流式响应为空")
	}

	// 成功：客户端响应 body 是 forwarder 可能做了模型名替换后的版本
	if ps.isDebugModeEnabled() {
		proxyCtx.ResponseBody = []byte(forwardResult.ResponseBody)
	}
	ps.finalizeAttempt(attempt, upstreamResp, true, "")

	promptTokens, completionTokens := ps.recordAttemptSuccess(c, proxyCtx, routeResult, forwardResult.ResponseBody, false)
	ps.logTraceInfo(proxyCtx.TraceID, "上游响应处理完成（非流式）",
		slog.Duration("duration", time.Since(proxyCtx.AttemptStartTime)),
		slog.Int64("prompt_tokens", promptTokens),
		slog.Int64("completion_tokens", completionTokens),
	)
	return nil
}

// markAttemptRetry 失败分支的 attempt 收尾（可重试错误使用）
func (ps *ProxyService) markAttemptRetry(
	attempt *AttemptRecord, resp *http.Response,
	failureType FailureType, failureScope FailureScope, stage, errMsg string,
) {
	if attempt == nil {
		return
	}
	attempt.FailureType = failureType
	attempt.FailureScope = failureScope
	attempt.FailureStage = stage
	ps.finalizeAttempt(attempt, resp, false, errMsg)
}

// readStreamSniffPayload 从流式响应体预读若干个 `data:` 帧用于嗅探。
// 返回预读 payload、首帧耗时（ms）、错误。
func (ps *ProxyService) readStreamSniffPayload(
	upstreamResp *http.Response,
	packetCount int,
	traceID string,
) ([]byte, int, error) {
	if packetCount <= 0 {
		packetCount = 1
	}

	startTime := time.Now()
	firstChunkMS := 0
	firstChunkSeen := false

	const maxProbeBytes = 256 * 1024
	buf := make([]byte, 1)
	payload := make([]byte, 0, 4096)
	var lineBuffer bytes.Buffer
	packetRead := 0

	for packetRead < packetCount {
		n, readErr := upstreamResp.Body.Read(buf)
		if n > 0 {
			if !firstChunkSeen {
				firstChunkSeen = true
				firstChunkMS = int(time.Since(startTime).Milliseconds())
			}

			ch := buf[0]
			payload = append(payload, ch)
			lineBuffer.WriteByte(ch)
			if ch == '\n' {
				line := strings.TrimSpace(lineBuffer.String())
				if strings.HasPrefix(line, "data:") {
					packetRead++
				}
				lineBuffer.Reset()
			}

			if len(payload) >= maxProbeBytes {
				break
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, firstChunkMS, readErr
		}
	}

	if len(payload) == 0 {
		return nil, firstChunkMS, &EmptySSEBodyError{
			TraceID: traceID,
			Message: "空流式响应体",
		}
	}

	return payload, firstChunkMS, nil
}

// recordAttemptSuccess 成功完成一次代理调用后的统计：
// 退出熔断冷却 → 解析 token → 异步写 token 统计 → 写请求指标 → 回填 proxyCtx。
// 返回本次的 prompt/completion token 数以便调用方继续记录日志。
func (ps *ProxyService) recordAttemptSuccess(
	c *gin.Context, proxyCtx *ProxyContext, routeResult *RouteResult,
	responseBody string, isStream bool,
) (int64, int64) {
	ps.recordSuccess(routeResult, proxyCtx.TraceID)
	promptTokens, completionTokens := proxyCtx.Endpoint.ParseTokenUsage(proxyCtx.RequestBody, responseBody, isStream)
	proxyCtx.PromptTokens = promptTokens
	proxyCtx.CompletionTokens = completionTokens
	ps.recordTokenUsageAsync(routeResult, getAccessTokenIDFromContext(c), promptTokens, completionTokens, proxyCtx.TraceID)
	ps.recordRequestMetrics(true, proxyCtx.Model, routeResult, promptTokens, completionTokens)
	return promptTokens, completionTokens
}

// =============================================================================
// 重试与故障归因
// =============================================================================

// tryScheduleRetry 记录故障、判断是否继续重试；超限时写最终错误响应与指标。
// 返回 true 表示已排程下一轮，调用方应 continue。
func (ps *ProxyService) tryScheduleRetry(
	c *gin.Context,
	proxyCtx *ProxyContext,
	routeResult *RouteResult,
	err error,
	failureType FailureType,
	failureScope FailureScope,
	stage string,
) bool {
	ps.recordFailure(routeResult, failureType, failureScope, err.Error(), proxyCtx.TraceID)
	ps.retryCoordinator.RecordAttempt(proxyCtx, routeResult.Channel.ID, routeResult.Channel.Name, routeResult.ModelConfigID, routeResult.Key.ID, err, failureType)
	proxyCtx.LastError = err
	proxyCtx.LastFailureType = failureType
	proxyCtx.LastFailureScope = failureScope
	proxyCtx.LastFailureStage = stage

	ps.logTraceInfo(proxyCtx.TraceID, "触发重试",
		slog.String("stage", stage),
		slog.Int("attempt", proxyCtx.AttemptCount),
		slog.String("failure_type", string(failureType)),
		slog.String("failure_scope", string(failureScope)),
		slog.String("error", err.Error()),
		slog.Uint64("channel_id", uint64(routeResult.Channel.ID)),
		slog.String("channel_name", routeResult.Channel.Name),
		slog.String("channel_model", routeResult.ChannelModel),
		slog.Uint64("key_id", uint64(routeResult.Key.ID)),
		slog.String("key_masked", maskSecret(routeResult.Key.ChannelKeyValue)),
	)

	if !ps.retryCoordinator.ShouldRetry(proxyCtx) {
		ps.responseForwarder.ForwardErrorResponse(c, http.StatusBadGateway, ErrAllUpstreamAttemptsFailed.Error(), proxyCtx.TraceID)
		ps.recordRequestMetrics(false, routeResult.Model, routeResult, 0, 0)
		return false
	}

	_ = ps.retryCoordinator.WaitBeforeRetry(c.Request.Context(), proxyCtx)
	return true
}

// recordFailure 将故障记录到熔断器，根据 scope/type 决定归因范围
func (ps *ProxyService) recordFailure(routeResult *RouteResult, failureType FailureType, failureScope FailureScope, errMsg, traceID string) {
	recordKey := failureScope == FailureScopeKey || failureScope == FailureScopeBoth
	recordModelConfig := failureScope == FailureScopeModelConfig || failureScope == FailureScopeBoth

	if recordKey {
		switch failureType {
		case FailureTypeHard:
			ps.circuitManager.RecordKeyHardFailure(routeResult.Key.ID, routeResult.Channel.ID, routeResult.Channel.Name, errMsg, traceID)
		case FailureTypeSoft:
			ps.circuitManager.RecordKeySoftFailure(routeResult.Key.ID, routeResult.Channel.ID, routeResult.Channel.Name, errMsg, traceID)
		}
	}
	if recordModelConfig && (failureType == FailureTypeSoft || failureType == FailureTypeModelNotFound) {
		ps.circuitManager.RecordModelConfigFailure(
			routeResult.ModelConfigID, routeResult.Channel.ID, routeResult.Channel.Name,
			routeResult.Model, routeResult.ChannelModel, errMsg, traceID,
		)
	}
}

// recordSuccess 成功后退出密钥/模型的冷却状态
func (ps *ProxyService) recordSuccess(routeResult *RouteResult, traceID string) {
	ps.circuitManager.RecordKeySuccess(routeResult.Key.ID, traceID)
	if routeResult.ModelConfigID != 0 {
		ps.circuitManager.RecordModelConfigSuccess(routeResult.ModelConfigID, routeResult.Channel.ID, traceID)
	}
}

// markClientCancelled 将 proxyCtx 标记为客户端取消；不记熔断、不重试、不计失败指标。
// 返回规范化后的错误（优先使用 request context err）。
func (ps *ProxyService) markClientCancelled(c *gin.Context, proxyCtx *ProxyContext, err error) error {
	cancelled := err
	if c != nil && c.Request != nil {
		if ctxErr := c.Request.Context().Err(); ctxErr != nil {
			cancelled = ctxErr
		}
	}
	if cancelled == nil {
		cancelled = context.Canceled
	}
	proxyCtx.LastError = cancelled
	proxyCtx.LastFailureStage = stageClientCancelled
	proxyCtx.LastFailureType = FailureTypeNone
	proxyCtx.LastFailureScope = FailureScopeNone
	return cancelled
}

// =============================================================================
// 指标 & Token 统计
// =============================================================================

func (ps *ProxyService) recordRequestMetrics(success bool, modelName string, routeResult *RouteResult, promptTokens, completionTokens int64) {
	event := metrics.RequestEvent{
		Timestamp:        time.Now(),
		Success:          success,
		Model:            modelName,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}
	if routeResult != nil {
		event.ChannelID = routeResult.Channel.ID
		event.ChannelName = routeResult.Channel.Name
	}
	ps.runtimeMetrics.Record(event)
}

func (ps *ProxyService) recordTokenUsageAsync(routeResult *RouteResult, accessTokenID uint, promptTokens, completionTokens int64, traceID string) {
	if promptTokens == 0 && completionTokens == 0 {
		return
	}
	ps.tokenUsageRecorder.Record(tokenUsageEvent{
		modelConfigID:    routeResult.ModelConfigID,
		keyID:            routeResult.Key.ID,
		accessTokenID:    accessTokenID,
		promptTokens:     promptTokens,
		completionTokens: completionTokens,
		traceID:          traceID,
	})
}
