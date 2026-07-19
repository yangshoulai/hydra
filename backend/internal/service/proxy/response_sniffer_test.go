package proxy

import (
	"io"
	"log/slog"
	"net/http"
	"testing"
)

func TestResponseSnifferDetectsFake200InSSEDataEvent(t *testing.T) {
	sniffer := NewResponseSniffer(slog.New(slog.NewTextHandler(io.Discard, nil)))
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	result, err := sniffer.Sniff(resp, true, []byte("data: {\"error\":{\"message\":\"invalid api key\"}}\n\n"), "test-trace")
	if err != nil {
		t.Fatalf("Sniff returned error: %v", err)
	}
	if result == nil || !result.IsFake200 || result.MatchedRule != "JSONErrorRule" {
		t.Fatalf("SSE fake-200 was not detected: %#v", result)
	}
}

func TestResponseSnifferDetectsFake200InPlainTextSSEDataEvent(t *testing.T) {
	sniffer := NewResponseSniffer(slog.New(slog.NewTextHandler(io.Discard, nil)))
	sniffer.UpdatePlainTextErrorKeywords([]string{"invalid api key"})
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	result, err := sniffer.Sniff(resp, true, []byte("data: error: invalid api key\n\n"), "test-trace")
	if err != nil {
		t.Fatalf("Sniff returned error: %v", err)
	}
	if result == nil || !result.IsFake200 || result.MatchedRule != "PlainTextErrorRule" {
		t.Fatalf("plain-text SSE fake-200 was not detected: %#v", result)
	}
}
