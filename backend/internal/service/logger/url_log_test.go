package logger

import (
	"net/url"
	"strings"
	"testing"
)

func TestSafeURLForLogRedactsSensitiveQueryValues(t *testing.T) {
	raw := "https://upstream.example/v1beta/models/gemini:generateContent?key=gemini-secret&alt=sse&api_key=another-secret"

	got := SafeURLForLog(raw)
	if strings.Contains(got, "gemini-secret") || strings.Contains(got, "another-secret") {
		t.Fatalf("sensitive query leaked: %s", got)
	}
	if !strings.Contains(got, "key="+redactedURLValue) || !strings.Contains(got, "api_key="+redactedURLValue) {
		t.Fatalf("sensitive query was not redacted: %s", got)
	}
	if !strings.Contains(got, "alt=sse") {
		t.Fatalf("non-sensitive query should be preserved: %s", got)
	}
}

func TestSafeURLValueForLogDoesNotMutateInput(t *testing.T) {
	parsed, err := url.Parse("https://upstream.example/path?key=secret")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	got := SafeURLValueForLog(parsed)
	if strings.Contains(got, "secret") {
		t.Fatalf("sensitive query leaked: %s", got)
	}
	if parsed.Query().Get("key") != "secret" {
		t.Fatalf("input URL was mutated: %s", parsed.String())
	}
}

func TestSafeURLForLogRedactsProxyPassword(t *testing.T) {
	got := SafeURLForLog("http://proxy-user:proxy-pass@proxy.local:8080")
	if strings.Contains(got, "proxy-pass") {
		t.Fatalf("proxy password leaked: %s", got)
	}
	if !strings.Contains(got, "proxy-user:"+redactedURLValue+"@") {
		t.Fatalf("proxy username should remain with password redacted: %s", got)
	}
}

func TestSafeURLForLogRedactsUserOnlyCredential(t *testing.T) {
	got := SafeURLForLog("https://token-only@proxy.local:8443")
	if strings.Contains(got, "token-only") {
		t.Fatalf("user-only credential leaked: %s", got)
	}
	if !strings.Contains(got, redactedURLValue+"@proxy.local") {
		t.Fatalf("user-only credential should be redacted: %s", got)
	}
}
