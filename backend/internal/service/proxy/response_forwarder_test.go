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
