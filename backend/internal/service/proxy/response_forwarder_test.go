package proxy

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestForwardStreamResponse_DoesNotInjectKeepaliveIntoNDJSON(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/stream", nil)

	reader, writer := io.Pipe()
	upstreamResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/x-ndjson"}},
		Body:       reader,
	}

	go func() {
		defer writer.Close()
		_, _ = writer.Write([]byte("{\"id\":1}\n"))
		time.Sleep(40 * time.Millisecond)
		_, _ = writer.Write([]byte("{\"id\":2}\n"))
	}()

	forwarder := NewResponseForwarder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	result, err := forwarder.ForwardStreamResponse(ctx, upstreamResp, "trace-ndjson", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("ForwardStreamResponse 返回错误: %v", err)
	}

	expectedBody := "{\"id\":1}\n{\"id\":2}\n"
	if got := recorder.Body.String(); got != expectedBody {
		t.Fatalf("非 SSE 流式响应不应注入保活帧，got=%q want=%q", got, expectedBody)
	}
	if result == nil || result.ResponseBody != expectedBody {
		t.Fatalf("返回结果异常: %+v", result)
	}
}

func TestForwardStreamResponse_InjectsKeepaliveIntoSSEOnly(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/stream", nil)

	reader, writer := io.Pipe()
	upstreamResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream; charset=utf-8"}},
		Body:       reader,
	}

	go func() {
		defer writer.Close()
		_, _ = writer.Write([]byte("data: first\n\n"))
		time.Sleep(40 * time.Millisecond)
		_, _ = writer.Write([]byte("data: second\n\n"))
	}()

	forwarder := NewResponseForwarder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	result, err := forwarder.ForwardStreamResponse(ctx, upstreamResp, "trace-sse", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("ForwardStreamResponse 返回错误: %v", err)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, ": keepalive\n\n") {
		t.Fatalf("SSE 流式响应应允许注入保活帧，body=%q", body)
	}
	if !strings.Contains(body, "data: first\n\n") || !strings.Contains(body, "data: second\n\n") {
		t.Fatalf("SSE 原始数据缺失，body=%q", body)
	}
	if result == nil || result.StreamChunks != 2 {
		t.Fatalf("流式分片统计异常: %+v", result)
	}
}
