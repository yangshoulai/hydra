package proxy

import (
	"net/http"
	"testing"
)

func TestClassifyHTTPErrorWithBodyDistinguishesAuthAndQuota(t *testing.T) {
	classifier := NewFailureClassifier()

	failureType, failureScope, _ := classifier.ClassifyHTTPErrorWithBody(
		http.StatusUnauthorized,
		[]byte(`{"error":{"message":"Incorrect API key provided"}}`),
		"application/json",
	)
	if failureType != FailureTypeHard || failureScope != FailureScopeKey {
		t.Fatalf("invalid key should be hard/key, got %s/%s", failureType, failureScope)
	}

	failureType, failureScope, _ = classifier.ClassifyHTTPErrorWithBody(
		http.StatusForbidden,
		[]byte(`{"error":{"message":"insufficient quota for this organization"}}`),
		"application/json",
	)
	if failureType != FailureTypeSoft || failureScope != FailureScopeKey {
		t.Fatalf("quota problem should be soft/key, got %s/%s", failureType, failureScope)
	}

	failureType, failureScope, _ = classifier.ClassifyHTTPErrorWithBody(
		http.StatusForbidden,
		[]byte(`{"error":{"message":"organization policy blocked this request"}}`),
		"application/json",
	)
	if failureType != FailureTypeSoft || failureScope != FailureScopeKey {
		t.Fatalf("unknown 403 should default to soft/key, got %s/%s", failureType, failureScope)
	}
}
