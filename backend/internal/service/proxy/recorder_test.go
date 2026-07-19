package proxy

import (
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/yangshoulai/hydra/internal/models"
)

func TestRequestLogRecorderCloseConcurrentRecord(t *testing.T) {
	recorder := &RequestLogRecorder{ch: make(chan RequestLogEvent, 8)}
	exerciseConcurrentClose(t, func() {
		recorder.Record(RequestLogEvent{Log: &models.RequestLog{}})
	}, recorder.Close)
}

func TestTokenUsageRecorderCloseConcurrentRecord(t *testing.T) {
	recorder := &TokenUsageRecorder{ch: make(chan tokenUsageEvent, 8)}
	exerciseConcurrentClose(t, func() {
		recorder.Record(tokenUsageEvent{promptTokens: 1})
	}, recorder.Close)
}

func TestRetryCoordinatorExcludesOnlyFailedRoutingUnit(t *testing.T) {
	coordinator := NewRetryCoordinator(slog.Default(), 3, 0, nil)
	proxyCtx := NewProxyContext("trace", "model", false, nil, nil)

	coordinator.RecordAttempt(proxyCtx, 11, "channel", 21, 31, errors.New("invalid key"), FailureTypeSoft, FailureScopeKey)
	if len(proxyCtx.FailedChannelIDs) != 0 || len(proxyCtx.FailedModelIDs) != 0 || len(proxyCtx.FailedKeyIDs) != 1 || proxyCtx.FailedKeyIDs[0] != 31 {
		t.Fatalf("key failure exclusions = channels:%v models:%v keys:%v", proxyCtx.FailedChannelIDs, proxyCtx.FailedModelIDs, proxyCtx.FailedKeyIDs)
	}

	coordinator.RecordAttempt(proxyCtx, 11, "channel", 21, 32, errors.New("model unavailable"), FailureTypeSoft, FailureScopeModelConfig)
	if len(proxyCtx.FailedChannelIDs) != 0 || len(proxyCtx.FailedModelIDs) != 1 || proxyCtx.FailedModelIDs[0] != 21 || len(proxyCtx.FailedKeyIDs) != 1 {
		t.Fatalf("model failure exclusions = channels:%v models:%v keys:%v", proxyCtx.FailedChannelIDs, proxyCtx.FailedModelIDs, proxyCtx.FailedKeyIDs)
	}
}

func exerciseConcurrentClose(t *testing.T, record func(), closeRecorder func()) {
	t.Helper()
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for range 200 {
				record()
			}
		}()
	}
	close(start)
	closeRecorder()
	wg.Wait()
}
