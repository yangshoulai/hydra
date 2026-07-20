package proxy

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestStreamCaptureRetainsTailWithinLimit(t *testing.T) {
	capture := newStreamCapture(5)
	capture.Write([]byte("abc"))
	capture.Write([]byte("defg"))

	if got := capture.String(); got != "cdefg" {
		t.Fatalf("capture tail = %q, want %q", got, "cdefg")
	}
	if !capture.Truncated() {
		t.Fatal("capture should be marked truncated")
	}
}

func TestStreamChunkCounterHandlesChunkBoundaries(t *testing.T) {
	counter := newStreamChunkCounter()
	counter.Observe([]byte(" data"))
	counter.Observe([]byte(": {\"id\":1}\r\nnot-data\n"))
	counter.Observe([]byte("\tdata: [DONE]\n"))

	if got := counter.Count(); got != 2 {
		t.Fatalf("data line count = %d, want 2", got)
	}
}

func TestStreamModelRewriterRewritesNestedModelsAcrossChunks(t *testing.T) {
	rewriter := newStreamModelRewriter("upstream-model", "public-model")
	first := rewriter.Transform([]byte("data: {\"model\":\"upstream-model\",\"response\":{\"model\":\"upstream-"), false)
	second := rewriter.Transform([]byte("model\"},\"untouched\":\"ok\"}\r\n"), true)
	got := string(append(first, second...))

	if !strings.HasSuffix(got, "\r\n") {
		t.Fatalf("line ending was not preserved: %q", got)
	}
	payload := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(got), "data:"))
	var data map[string]any
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		t.Fatalf("rewritten payload is invalid JSON: %v", err)
	}
	if gotModel := data["model"]; gotModel != "public-model" {
		t.Fatalf("root model = %#v, want public-model", gotModel)
	}
	response, ok := data["response"].(map[string]any)
	if !ok || response["model"] != "public-model" {
		t.Fatalf("nested model = %#v, want public-model", data["response"])
	}
}

func TestSSEModelRewriterHasBoundedPendingBuffer(t *testing.T) {
	rewriter := newStreamModelRewriter("upstream-model", "public-model")
	oversized := []byte("data: " + strings.Repeat("x", maxSSERewritePendingBytes))
	got := rewriter.Transform(oversized, false)

	if string(got) != string(oversized) {
		t.Fatal("oversized line should be forwarded unchanged")
	}
	if !rewriter.disabled || len(rewriter.pending) != 0 {
		t.Fatalf("rewriter state after oversized line = disabled:%v pending:%d", rewriter.disabled, len(rewriter.pending))
	}
}

func TestShouldSendSSEKeepalive(t *testing.T) {
	if !shouldSendSSEKeepalive("text/event-stream; charset=utf-8") {
		t.Fatal("SSE content type should enable keepalive")
	}
	if shouldSendSSEKeepalive("application/json") {
		t.Fatal("JSON stream should not receive SSE keepalive frames")
	}
}

func TestSSEEventBoundaryTrackerWaitsForCompleteEventBoundary(t *testing.T) {
	tracker := newSSEEventBoundaryTracker()
	if !tracker.CanWriteKeepalive() {
		t.Fatal("a new stream should be safe before any payload is written")
	}

	tracker.Observe([]byte("data: {\"message\":\"partial"))
	if tracker.CanWriteKeepalive() {
		t.Fatal("a partial data line must not allow a keepalive")
	}

	tracker.Observe([]byte(" value\"}\r"))
	if tracker.CanWriteKeepalive() {
		t.Fatal("a trailing CR must wait for its possible LF")
	}
	tracker.Observe([]byte("\n"))
	if tracker.CanWriteKeepalive() {
		t.Fatal("the first CRLF only ends the data line, not the SSE event")
	}
	tracker.Observe([]byte("\r\n"))
	if !tracker.CanWriteKeepalive() {
		t.Fatal("a completed blank line should allow a keepalive")
	}
}

func TestCaptureStreamForwardDebugResponseIncludesPartialStreamAndKeepalive(t *testing.T) {
	service := &ProxyService{}
	service.debugModeEnabled.Store(true)
	proxyCtx := &ProxyContext{}
	attempt := &AttemptRecord{}
	forwardResult := &StreamForwardResult{
		ResponseBody: "data: {\"part\":1}\n\n: keepalive\n\n",
	}

	service.captureStreamForwardDebugResponse(proxyCtx, attempt, forwardResult)

	if got, want := string(proxyCtx.ResponseBody), forwardResult.ResponseBody; got != want {
		t.Fatalf("client debug body = %q, want %q", got, want)
	}
	if got, want := string(attempt.UpstreamResponseBody), forwardResult.ResponseBody; got != want {
		t.Fatalf("attempt debug body = %q, want %q", got, want)
	}
}

func TestForwardStreamResponseDoesNotInsertKeepaliveIntoPartialSSEData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	first := []byte("data: {\"message\":\"partial")
	second := []byte(" value\"}\n\n")
	body := newDelayedStreamReadCloser(first, second)
	upstreamResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       body,
	}

	type forwardResult struct {
		result *StreamForwardResult
		err    error
	}
	resultCh := make(chan forwardResult, 1)
	go func() {
		result, err := NewResponseForwarder(slog.New(slog.NewTextHandler(io.Discard, nil))).ForwardStreamResponse(
			c, upstreamResp, "same-model", "same-model", "trace", 10*time.Millisecond, 0,
		)
		resultCh <- forwardResult{result: result, err: err}
	}()
	defer body.Release()

	select {
	case <-body.firstRead:
	case <-time.After(time.Second):
		t.Fatal("stream did not read the first partial SSE payload")
	}
	// Give the keepalive timer multiple opportunities to fire while the SSE data
	// line is incomplete. It must not inject a comment into that line.
	time.Sleep(80 * time.Millisecond)
	body.Release()

	select {
	case forwarded := <-resultCh:
		if forwarded.err != nil {
			t.Fatalf("ForwardStreamResponse: %v", forwarded.err)
		}
		if forwarded.result == nil {
			t.Fatal("ForwardStreamResponse returned a nil result")
		}
	case <-time.After(time.Second):
		t.Fatal("stream did not finish after the remaining payload was released")
	}

	if got, want := recorder.Body.String(), string(append(first, second...)); got != want {
		t.Fatalf("forwarded body = %q, want %q", got, want)
	}
}

func TestForwardStreamResponseKeepsAliveBetweenCompleteSSEEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	first := []byte("data: {\"part\":1}\n\n")
	second := []byte("data: {\"part\":2}\n\n")
	body := newDelayedStreamReadCloser(first, second)
	upstreamResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       body,
	}

	type forwardResult struct {
		result *StreamForwardResult
		err    error
	}
	resultCh := make(chan forwardResult, 1)
	go func() {
		result, err := NewResponseForwarder(slog.New(slog.NewTextHandler(io.Discard, nil))).ForwardStreamResponse(
			c, upstreamResp, "same-model", "same-model", "trace", 10*time.Millisecond, 0,
		)
		resultCh <- forwardResult{result: result, err: err}
	}()
	defer body.Release()

	var captured *StreamForwardResult
	select {
	case <-body.firstRead:
	case <-time.After(time.Second):
		t.Fatal("stream did not read the first SSE event")
	}
	time.Sleep(80 * time.Millisecond)
	body.Release()

	select {
	case forwarded := <-resultCh:
		if forwarded.err != nil {
			t.Fatalf("ForwardStreamResponse: %v", forwarded.err)
		}
		if forwarded.result == nil {
			t.Fatal("ForwardStreamResponse returned a nil result")
		}
		captured = forwarded.result
	case <-time.After(time.Second):
		t.Fatal("stream did not finish after the remaining event was released")
	}

	got := recorder.Body.String()
	if !strings.Contains(got, string(first)) || !strings.Contains(got, string(second)) {
		t.Fatalf("forwarded body should retain both SSE events: %q", got)
	}
	if !strings.Contains(got, ": keepalive\n\n") {
		t.Fatalf("forwarded body should contain a keepalive between complete events: %q", got)
	}
	if captured == nil || captured.ResponseBody != got {
		t.Fatalf("debug capture = %#v, want the complete client-visible body %q", captured, got)
	}
}

func TestForwardStreamResponseRetainsPartialBodyOnReadError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	payload := []byte("data: {\"part\":1}\n\n")
	readErr := errors.New("upstream connection closed")
	upstreamResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       &errorAfterChunkReadCloser{part: payload, err: readErr},
	}

	result, err := NewResponseForwarder(slog.New(slog.NewTextHandler(io.Discard, nil))).ForwardStreamResponse(
		c, upstreamResp, "same-model", "same-model", "trace", 0, 0,
	)
	if !errors.Is(err, readErr) {
		t.Fatalf("ForwardStreamResponse error = %v, want %v", err, readErr)
	}
	if result == nil || result.ResponseBody != string(payload) {
		t.Fatalf("partial stream capture = %#v, want %q", result, payload)
	}
}

func TestForwardStreamResponseDoesNotCommitBeforeFirstPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	upstreamResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("")),
	}
	result, err := NewResponseForwarder(slog.New(slog.NewTextHandler(io.Discard, nil))).ForwardStreamResponse(
		c, upstreamResp, "upstream-model", "public-model", "trace", 0, 0,
	)

	var emptyErr *EmptySSEBodyError
	if !errors.As(err, &emptyErr) {
		t.Fatalf("error = %v, want EmptySSEBodyError", err)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
	if c.Writer.Written() {
		t.Fatal("empty upstream stream must remain uncommitted so the caller can retry")
	}
}

func TestForwardStreamResponseTimesOutBeforeCommit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	upstreamResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       newBlockingReadCloser(),
	}
	result, err := NewResponseForwarder(slog.New(slog.NewTextHandler(io.Discard, nil))).ForwardStreamResponse(
		c, upstreamResp, "upstream-model", "public-model", "trace", 0, 10*time.Millisecond,
	)
	if !errors.Is(err, ErrUpstreamStreamIdle) {
		t.Fatalf("error = %v, want ErrUpstreamStreamIdle", err)
	}
	if result == nil || result.ResponseCommitted {
		t.Fatalf("result = %#v, expected an uncommitted result", result)
	}
	if c.Writer.Written() {
		t.Fatal("idle timeout before first payload must remain retryable")
	}
}

func TestStreamSniffReadUsesIdleTimeout(t *testing.T) {
	service := &ProxyService{}
	service.updateStreamIdleTimeout(10 * time.Millisecond)
	upstreamResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       newBlockingReadCloser(),
	}

	_, _, err := service.readStreamSniffPayload(upstreamResp, 1, "trace")
	if !errors.Is(err, ErrUpstreamStreamIdle) {
		t.Fatalf("error = %v, want ErrUpstreamStreamIdle", err)
	}
}

func TestReadResponseBodyEnforcesLimit(t *testing.T) {
	_, err := readResponseBody(strings.NewReader("12345"), 4)
	if !errors.Is(err, ErrUpstreamResponseTooLarge) {
		t.Fatalf("error = %v, want ErrUpstreamResponseTooLarge", err)
	}
}

func TestForwardNonStreamResponsePreservesRetryHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	upstreamResp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"Retry-After":  []string{"3"},
		},
	}
	upstreamResp.Header.Set("X-RateLimit-Remaining-Requests", "0")

	_, err := NewResponseForwarder(slog.New(slog.NewTextHandler(io.Discard, nil))).ForwardNonStreamResponse(
		c, upstreamResp, []byte(`{"error":"rate limited"}`), "", "", "trace",
	)
	if err != nil {
		t.Fatalf("ForwardNonStreamResponse: %v", err)
	}
	if got := recorder.Header().Get("Retry-After"); got != "3" {
		t.Fatalf("Retry-After = %q, want 3", got)
	}
	if got := recorder.Header().Get("X-RateLimit-Remaining-Requests"); got != "0" {
		t.Fatalf("rate limit header = %q, want 0", got)
	}
}

type blockingReadCloser struct {
	done chan struct{}
	once sync.Once
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{done: make(chan struct{})}
}

func (r *blockingReadCloser) Read(_ []byte) (int, error) {
	<-r.done
	return 0, io.EOF
}

func (r *blockingReadCloser) Close() error {
	r.once.Do(func() { close(r.done) })
	return nil
}

type delayedStreamReadCloser struct {
	first     []byte
	second    []byte
	readCount int
	firstRead chan struct{}
	release   chan struct{}
	once      sync.Once
}

func newDelayedStreamReadCloser(first, second []byte) *delayedStreamReadCloser {
	return &delayedStreamReadCloser{
		first:     first,
		second:    second,
		firstRead: make(chan struct{}),
		release:   make(chan struct{}),
	}
}

func (r *delayedStreamReadCloser) Read(p []byte) (int, error) {
	switch r.readCount {
	case 0:
		r.readCount++
		close(r.firstRead)
		return copy(p, r.first), nil
	case 1:
		<-r.release
		r.readCount++
		return copy(p, r.second), nil
	default:
		return 0, io.EOF
	}
}

func (r *delayedStreamReadCloser) Release() {
	r.once.Do(func() { close(r.release) })
}

func (r *delayedStreamReadCloser) Close() error {
	r.Release()
	return nil
}

type errorAfterChunkReadCloser struct {
	part []byte
	err  error
	read bool
}

func (r *errorAfterChunkReadCloser) Read(p []byte) (int, error) {
	if r.read {
		return 0, r.err
	}
	r.read = true
	return copy(p, r.part), nil
}

func (r *errorAfterChunkReadCloser) Close() error {
	return nil
}
