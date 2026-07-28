package proxy

import (
	"net/http"
	"strings"
	"testing"

	"github.com/yangshoulai/hydra/internal/endpoint"
)

func TestRequestBodyForDebugLogOmitsImageEditMultipart(t *testing.T) {
	body := []byte("binary-png-payload")
	headers := http.Header{"Content-Type": []string{"multipart/form-data; boundary=test"}}

	logged := requestBodyForDebugLog(body, headers, endpoint.TypeOpenAIImagesEdits)
	if strings.Contains(logged, string(body)) {
		t.Fatalf("binary body leaked into log: %q", logged)
	}
	if !strings.Contains(logged, "multipart/form-data") || !strings.Contains(logged, "18 bytes") {
		t.Fatalf("missing safe metadata: %q", logged)
	}
}

func TestRequestBodyForDebugLogListsJSONFieldsWithoutValues(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","prompt":"secret prompt","image":"secret-image-data"}`)
	headers := http.Header{"Content-Type": []string{"application/json"}}

	logged := requestBodyForDebugLog(body, headers, endpoint.TypeOpenAIImagesEdits)
	if strings.Contains(logged, "secret prompt") || strings.Contains(logged, "secret-image-data") {
		t.Fatalf("image edit values leaked into log: %q", logged)
	}
	if !strings.Contains(logged, "fields=image,model,prompt") {
		t.Fatalf("missing JSON field summary: %q", logged)
	}
}
