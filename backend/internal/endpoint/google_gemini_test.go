package endpoint

import (
	"bytes"
	"net/http"
	"testing"
)

func TestGeminiConfigureRequestOverridesInboundQueryKey(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://upstream.example/v1beta/models/old-model:generateContent?key=hydra-access-token&alt=sse", bytes.NewBufferString(`{"contents":[]}`))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	body, err := (&GeminiEndpoint{}).ConfigureRequest(req, "channel-key", "gemini-2.5-pro", []byte(`{"contents":[]}`))
	if err != nil {
		t.Fatalf("ConfigureRequest: %v", err)
	}
	if string(body) != `{"contents":[]}` {
		t.Fatalf("request body changed unexpectedly: %s", body)
	}
	if got := req.URL.Query().Get("key"); got != "channel-key" {
		t.Fatalf("upstream query key = %q, want channel-key", got)
	}
	if got := req.Header.Get("X-Goog-Api-Key"); got != "channel-key" {
		t.Fatalf("upstream X-Goog-Api-Key = %q, want channel-key", got)
	}
	if got := req.URL.Path; got != "/v1beta/models/gemini-2.5-pro:generateContent" {
		t.Fatalf("upstream path = %q", got)
	}
}
