package proxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	configService "github.com/yangshoulai/hydra/internal/service/config"
)

type nonStreamResponseResult struct {
	resp *http.Response
	err  error
}

type nonStreamBodyResult struct {
	body []byte
	err  error
}

type nonStreamKeepaliveController struct {
	ps       *ProxyService
	c        *gin.Context
	proxyCtx *ProxyContext
	cfg      configService.NonStreamKeepaliveConfig

	committed bool

	firstTimer *time.Timer
	firstC     <-chan time.Time
	ticker     *time.Ticker
	tickerC    <-chan time.Time
}

func newNonStreamKeepaliveController(
	ps *ProxyService,
	c *gin.Context,
	proxyCtx *ProxyContext,
	cfg configService.NonStreamKeepaliveConfig,
) *nonStreamKeepaliveController {
	delay := cfg.FirstDelay - time.Since(proxyCtx.StartTime)
	if delay < 0 {
		delay = 0
	}

	firstTimer := time.NewTimer(delay)
	return &nonStreamKeepaliveController{
		ps:         ps,
		c:          c,
		proxyCtx:   proxyCtx,
		cfg:        cfg,
		firstTimer: firstTimer,
		firstC:     firstTimer.C,
	}
}

func (kc *nonStreamKeepaliveController) stop() {
	if kc.firstTimer != nil {
		if !kc.firstTimer.Stop() {
			select {
			case <-kc.firstTimer.C:
			default:
			}
		}
	}
	if kc.ticker != nil {
		kc.ticker.Stop()
	}
}

func (kc *nonStreamKeepaliveController) waitForResponse(resultCh <-chan nonStreamResponseResult) (nonStreamResponseResult, error) {
	for {
		select {
		case <-kc.c.Request.Context().Done():
			return nonStreamResponseResult{}, kc.c.Request.Context().Err()
		case <-kc.firstC:
			if err := kc.handleKeepaliveTick(); err != nil {
				return nonStreamResponseResult{}, err
			}
		case <-kc.tickerC:
			if err := kc.writeKeepaliveWhitespace(); err != nil {
				return nonStreamResponseResult{}, err
			}
		case result := <-resultCh:
			return result, nil
		}
	}
}

func (kc *nonStreamKeepaliveController) waitForBody(resultCh <-chan nonStreamBodyResult) (nonStreamBodyResult, error) {
	for {
		select {
		case <-kc.c.Request.Context().Done():
			return nonStreamBodyResult{}, kc.c.Request.Context().Err()
		case <-kc.firstC:
			if err := kc.handleKeepaliveTick(); err != nil {
				return nonStreamBodyResult{}, err
			}
		case <-kc.tickerC:
			if err := kc.writeKeepaliveWhitespace(); err != nil {
				return nonStreamBodyResult{}, err
			}
		case result := <-resultCh:
			return result, nil
		}
	}
}

func (kc *nonStreamKeepaliveController) handleKeepaliveTick() error {
	kc.firstC = nil
	if kc.firstTimer != nil {
		if !kc.firstTimer.Stop() {
			select {
			case <-kc.firstTimer.C:
			default:
			}
		}
		kc.firstTimer = nil
	}

	if err := kc.writeKeepaliveWhitespace(); err != nil {
		return err
	}

	if kc.cfg.Interval > 0 {
		kc.ticker = time.NewTicker(kc.cfg.Interval)
		kc.tickerC = kc.ticker.C
	}
	return nil
}

func (kc *nonStreamKeepaliveController) writeKeepaliveWhitespace() error {
	flusher, ok := kc.c.Writer.(http.Flusher)
	if !ok {
		return io.ErrUnexpectedEOF
	}

	if !kc.committed {
		kc.c.Header("Content-Type", "application/json; charset=utf-8")
		kc.c.Header("Cache-Control", "no-cache, no-transform")
		kc.c.Header("X-Accel-Buffering", "no")
		kc.c.Status(http.StatusOK)
		kc.proxyCtx.ResponseStatusLocked = true
		kc.committed = true
		kc.ps.logTraceInfo(kc.proxyCtx.TraceID, "非流式保活已提交响应状态码",
			slog.Duration("first_delay", kc.cfg.FirstDelay),
			slog.Duration("interval", kc.cfg.Interval),
			slog.Duration("elapsed", time.Since(kc.proxyCtx.StartTime)),
		)
	}

	if _, err := kc.c.Writer.Write([]byte("\n")); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// handleNonStreamAttemptWithKeepalive 为非流式请求提供 JSON whitespace 保活。
// 保活写出前仍保持原有语义：可以按嗅探/校验结果重试；
// 保活写出后 HTTP 状态码已提交，后续失败只写错误 JSON body，不再重试。
func (ps *ProxyService) handleNonStreamAttemptWithKeepalive(
	c *gin.Context,
	proxyCtx *ProxyContext,
	routeResult *RouteResult,
	upstreamReq *http.Request,
	cfg configService.NonStreamKeepaliveConfig,
) error {
	attempt := proxyCtx.CurrentAttempt()
	keepalive := newNonStreamKeepaliveController(ps, c, proxyCtx, cfg)
	defer keepalive.stop()

	stopCh := make(chan struct{})
	defer close(stopCh)

	respCh := make(chan nonStreamResponseResult)
	go func() {
		resp, err := ps.httpClient.DoWithProxy(upstreamReq, proxyCtx.TraceID, routeResult.Channel.UseProxy)
		result := nonStreamResponseResult{resp: resp, err: err}
		select {
		case respCh <- result:
		case <-stopCh:
			drainAndCloseBody(resp)
		}
	}()

	respResult, waitErr := keepalive.waitForResponse(respCh)
	if waitErr != nil {
		return ps.handleNonStreamKeepaliveWaitError(c, proxyCtx, attempt, waitErr)
	}

	upstreamResp := respResult.resp
	if respResult.err == nil && upstreamResp == nil {
		respResult.err = ErrUpstreamCallFailed
	}

	if isClientCancelled(c, respResult.err) {
		if upstreamResp != nil {
			ps.captureUpstreamResponse(attempt, upstreamResp, false)
		}
		drainAndCloseBody(upstreamResp)
		ps.finalizeAttempt(attempt, upstreamResp, false, "client cancelled")
		return ps.markClientCancelled(c, proxyCtx, respResult.err)
	}

	failureType, failureScope, errMsg := ps.failureClassifier.ClassifyResponseError(upstreamResp, respResult.err)
	if failureType != FailureTypeNone {
		ps.captureUpstreamResponse(attempt, upstreamResp, ps.isDebugModeEnabled() && !keepalive.committed)
		drainAndCloseBody(upstreamResp)
		if attempt != nil {
			attempt.FailureType = failureType
			attempt.FailureScope = failureScope
		}
		ps.finalizeAttempt(attempt, upstreamResp, false, errMsg)

		cause := NewUpstreamCallError(errMsg)
		if keepalive.committed {
			return ps.finishNonStreamCommittedFailure(
				c, proxyCtx, routeResult, attempt, upstreamResp,
				cause, failureType, failureScope, stageNonStreamKeepalive, errMsg,
			)
		}
		return NewRetryableProxyError(cause, failureType, failureScope, "上游调用失败")
	}

	if attempt != nil {
		attempt.UpstreamResponseHeaders = upstreamResp.Header.Clone()
		attempt.UpstreamStatusCode = upstreamResp.StatusCode
	}

	bodyCh := make(chan nonStreamBodyResult)
	go func(body io.ReadCloser) {
		bodyBytes, readErr := readResponseBody(body, ps.getMaxResponseBytes())
		_ = body.Close()
		result := nonStreamBodyResult{body: bodyBytes, err: readErr}
		select {
		case bodyCh <- result:
		case <-stopCh:
		}
	}(upstreamResp.Body)

	bodyResult, waitErr := keepalive.waitForBody(bodyCh)
	if waitErr != nil {
		_ = upstreamResp.Body.Close()
		return ps.handleNonStreamKeepaliveWaitError(c, proxyCtx, attempt, waitErr)
	}

	return ps.finishNonStreamKeepaliveBody(c, proxyCtx, routeResult, upstreamResp, bodyResult.body, bodyResult.err, keepalive.committed)
}

func (ps *ProxyService) handleNonStreamKeepaliveWaitError(
	c *gin.Context,
	proxyCtx *ProxyContext,
	attempt *AttemptRecord,
	err error,
) error {
	if isClientCancelled(c, err) || errorsIsContextDone(err) {
		ps.finalizeAttempt(attempt, nil, false, "client cancelled")
		return ps.markClientCancelled(c, proxyCtx, err)
	}

	ps.logTraceDebug(proxyCtx.TraceID, "非流式保活等待失败", slog.String("error", err.Error()))
	proxyCtx.LastError = err
	proxyCtx.LastFailureStage = stageNonStreamKeepalive
	ps.finalizeAttempt(attempt, nil, false, err.Error())
	ps.recordRequestMetrics(false, proxyCtx.Model, nil, 0, 0)
	return err
}

func errorsIsContextDone(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (ps *ProxyService) finishNonStreamKeepaliveBody(
	c *gin.Context,
	proxyCtx *ProxyContext,
	routeResult *RouteResult,
	upstreamResp *http.Response,
	body []byte,
	readErr error,
	committed bool,
) error {
	attempt := proxyCtx.CurrentAttempt()

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
		if committed {
			return ps.finishNonStreamCommittedFailure(
				c, proxyCtx, routeResult, attempt, upstreamResp,
				readErr, FailureTypeNone, FailureScopeNone, stageNonStreamRead, readErr.Error(),
			)
		}
		return readErr
	}

	if attempt != nil && ps.isDebugModeEnabled() {
		attempt.UpstreamResponseBody = append([]byte(nil), body...)
	}

	if ps.isNonStreamSnifferEnabled() {
		sniffResult, sniffErr := ps.responseSniffer.Sniff(upstreamResp, false, body, proxyCtx.TraceID)
		if sniffErr != nil {
			if committed {
				return ps.finishNonStreamCommittedFailure(
					c, proxyCtx, routeResult, attempt, upstreamResp,
					sniffErr, FailureTypeSoft, FailureScopeNone, stageNonStreamRead, sniffErr.Error(),
				)
			}
			ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeNone, stageNonStreamRead, sniffErr.Error())
			return NewRetryableProxyError(sniffErr, FailureTypeSoft, FailureScopeNone, "非流式嗅探失败")
		}
		if sniffResult == nil {
			if committed {
				return ps.finishNonStreamCommittedFailure(
					c, proxyCtx, routeResult, attempt, upstreamResp,
					ErrEmptySniffResult, FailureTypeSoft, FailureScopeNone, stageNonStreamRead, ErrEmptySniffResult.Error(),
				)
			}
			ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeNone, stageNonStreamRead, ErrEmptySniffResult.Error())
			return NewRetryableProxyError(ErrEmptySniffResult, FailureTypeSoft, FailureScopeNone, "非流式嗅探结果为空")
		}
		if sniffResult.IsFake200 {
			if committed {
				return ps.finishNonStreamCommittedFailure(
					c, proxyCtx, routeResult, attempt, upstreamResp,
					ErrFake200Response, FailureTypeSoft, FailureScopeKey, stageNonStreamRead, ErrFake200Response.Error(),
				)
			}
			ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeKey, stageNonStreamRead, ErrFake200Response.Error())
			return NewRetryableProxyError(ErrFake200Response, FailureTypeSoft, FailureScopeKey, "非流式假200")
		}
	}

	if valid, validateMsg := proxyCtx.Endpoint.ValidateResponse(upstreamResp.StatusCode, body); !valid {
		validationErr := NewResponseValidationError(validateMsg)
		if committed {
			return ps.finishNonStreamCommittedFailure(
				c, proxyCtx, routeResult, attempt, upstreamResp,
				validationErr, FailureTypeSoft, FailureScopeModelConfig, stageNonStreamRead, validateMsg,
			)
		}
		ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeModelConfig, stageNonStreamRead, validateMsg)
		return NewRetryableProxyError(validationErr, FailureTypeSoft, FailureScopeModelConfig, "响应校验失败")
	}

	var (
		forwardResult *NonStreamForwardResult
		forwardErr    error
	)
	if committed {
		preparedBody := ps.responseForwarder.prepareNonStreamBody(
			upstreamResp, body, routeResult.ChannelModel, routeResult.Model, proxyCtx.TraceID,
		)
		forwardResult, forwardErr = ps.responseForwarder.ForwardLockedNonStreamBody(c, preparedBody)
	} else {
		forwardResult, forwardErr = ps.responseForwarder.ForwardNonStreamResponse(
			c, upstreamResp, body, routeResult.ChannelModel, routeResult.Model, proxyCtx.TraceID,
		)
	}

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
		if committed {
			return ps.finishNonStreamCommittedFailure(
				c, proxyCtx, routeResult, attempt, upstreamResp,
				ErrEmptySniffResult, FailureTypeSoft, FailureScopeNone, stageNonStreamForward, "非流式响应为空",
			)
		}
		ps.markAttemptRetry(attempt, upstreamResp, FailureTypeSoft, FailureScopeNone, stageNonStreamForward, ErrEmptySniffResult.Error())
		return NewRetryableProxyError(ErrEmptySniffResult, FailureTypeSoft, FailureScopeNone, "非流式响应为空")
	}

	if ps.isDebugModeEnabled() {
		proxyCtx.ResponseBody = []byte(forwardResult.ResponseBody)
	}
	ps.finalizeAttempt(attempt, upstreamResp, true, "")

	promptTokens, completionTokens := ps.recordAttemptSuccess(c, proxyCtx, routeResult, forwardResult.ResponseBody, false)
	ps.logTraceInfo(proxyCtx.TraceID, "上游响应处理完成（非流式保活路径）",
		slog.Int("attempt", attemptNumber(attempt)),
		slog.Duration("duration", time.Since(proxyCtx.AttemptStartTime)),
		slog.Bool("keepalive_committed", committed),
		slog.Int64("prompt_tokens", promptTokens),
		slog.Int64("completion_tokens", completionTokens),
	)
	return nil
}

func (ps *ProxyService) finishNonStreamCommittedFailure(
	c *gin.Context,
	proxyCtx *ProxyContext,
	routeResult *RouteResult,
	attempt *AttemptRecord,
	upstreamResp *http.Response,
	cause error,
	failureType FailureType,
	failureScope FailureScope,
	stage string,
	message string,
) error {
	if cause == nil {
		cause = ErrUpstreamCallFailed
	}
	if message == "" {
		message = cause.Error()
	}

	if failureType != FailureTypeNone {
		ps.recordFailure(routeResult, failureType, failureScope, message, proxyCtx.TraceID)
	}

	if attempt != nil {
		attempt.FailureType = failureType
		attempt.FailureScope = failureScope
		attempt.FailureStage = stage
		ps.finalizeAttempt(attempt, upstreamResp, false, message)
	}

	proxyCtx.LastError = cause
	proxyCtx.LastFailureType = failureType
	proxyCtx.LastFailureScope = failureScope
	proxyCtx.LastFailureStage = stage
	ps.recordRequestMetrics(false, proxyCtx.Model, routeResult, 0, 0)

	responseBody, writeErr := ps.responseForwarder.ForwardLockedErrorBody(c, message, proxyCtx.TraceID)
	if ps.isDebugModeEnabled() {
		proxyCtx.ResponseBody = []byte(responseBody)
	}
	if writeErr != nil {
		if isClientCancelled(c, writeErr) {
			return ps.markClientCancelled(c, proxyCtx, writeErr)
		}
		ps.logTraceDebug(proxyCtx.TraceID, "状态码锁定后写出错误响应失败", slog.String("error", writeErr.Error()))
		return writeErr
	}

	ps.logTraceInfo(proxyCtx.TraceID, "非流式保活已锁定状态码，失败不再重试",
		slog.String("stage", stage),
		slog.String("failure_type", string(failureType)),
		slog.String("failure_scope", string(failureScope)),
		slog.String("error", cause.Error()),
	)
	return cause
}
