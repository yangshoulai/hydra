package logger

import (
	"strings"
	"testing"
)

func TestSafeResponseBodyForLogRedactsB64JSON(t *testing.T) {
	body := []byte(`{"data":[{"b64_json":"` + strings.Repeat("A", 512) + `","revised_prompt":"测试"}]}`)

	preview, truncated, redacted := SafeResponseBodyForLog(body)

	if !redacted {
		t.Fatal("expected image base64 to be redacted")
	}
	if truncated {
		t.Fatal("redacted body should not need truncation")
	}
	if strings.Contains(preview, strings.Repeat("A", 128)) {
		t.Fatal("preview still contains raw base64 payload")
	}
	if !strings.Contains(preview, "[omitted base64 image, chars=512]") {
		t.Fatalf("preview missing redaction marker: %s", preview)
	}
}

func TestSafeResponseBodyForLogRedactsDataImageURL(t *testing.T) {
	body := []byte(`{"image":"data:image/png;base64,` + strings.Repeat("B", 256) + `"}`)

	preview, truncated, redacted := SafeResponseBodyForLog(body)

	if !redacted {
		t.Fatal("expected data image URL to be redacted")
	}
	if truncated {
		t.Fatal("redacted data image URL should not need truncation")
	}
	if strings.Contains(preview, strings.Repeat("B", 128)) {
		t.Fatal("preview still contains raw data image payload")
	}
	if !strings.Contains(preview, "data:image/png;base64,[omitted base64 image, chars=256]") {
		t.Fatalf("preview missing data image redaction marker: %s", preview)
	}
}

func TestSafeResponseBodyForLogTruncatesLargeText(t *testing.T) {
	body := []byte(strings.Repeat("响应", maxResponseBodyLogRunes+10))

	preview, truncated, redacted := SafeResponseBodyForLog(body)

	if redacted {
		t.Fatal("plain text response should not be marked redacted")
	}
	if !truncated {
		t.Fatal("expected large response to be truncated")
	}
	if !strings.Contains(preview, "[truncated, original_bytes=") {
		t.Fatalf("preview missing truncation marker: %s", preview)
	}
}
