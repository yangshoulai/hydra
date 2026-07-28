package endpoint

import (
	"net/http"
	"strings"
	"testing"
)

func TestResponsesEndpointAllowsQueuedBackgroundResponse(t *testing.T) {
	valid, message := (&ResponsesEndpoint{}).ValidateResponse(200, []byte(`{"id":"resp_123","status":"queued","output":[]}`))
	if !valid {
		t.Fatalf("queued background response rejected: %s", message)
	}
}

func TestGeminiEndpointAllowsSafetyFeedbackWithoutCandidates(t *testing.T) {
	valid, message := (&GeminiEndpoint{}).ValidateResponse(200, []byte(`{"promptFeedback":{"blockReason":"SAFETY"}}`))
	if !valid {
		t.Fatalf("safety feedback response rejected: %s", message)
	}
}

func TestImagesEditEndpointReturnsUnsupportedContentTypeError(t *testing.T) {
	request, err := http.NewRequest(http.MethodPost, "https://upstream.example/v1/images/edits", strings.NewReader(`{"model":"public"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", "text/plain")
	_, err = (&ImagesEditEndpoint{}).ConfigureRequest(request, "key", "gpt-image-1", []byte(`{"model":"public"}`))
	if err == nil {
		t.Fatal("unsupported content type must return an error instead of silently preserving the public model")
	}
}
