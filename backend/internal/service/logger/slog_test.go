package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRuntimeHandlerSwitchesLogFormat(t *testing.T) {
	var buf bytes.Buffer
	handler := &runtimeHandler{
		text: slog.NewTextHandler(&buf, nil),
		json: slog.NewJSONHandler(&buf, nil),
	}
	testLogger := slog.New(handler)

	SetLogFormat(LogFormatJSON)
	testLogger.Info("格式切换测试", slog.String("kind", "json"))
	if got := buf.String(); !strings.Contains(got, `"msg":"格式切换测试"`) || !strings.Contains(got, `"kind":"json"`) {
		t.Fatalf("expected JSON log output, got: %s", got)
	}

	buf.Reset()
	SetLogFormat(LogFormatText)
	testLogger.Info("格式切换测试", slog.String("kind", "text"))
	if got := buf.String(); !strings.Contains(got, "msg=格式切换测试") || !strings.Contains(got, "kind=text") {
		t.Fatalf("expected text log output, got: %s", got)
	}
}
