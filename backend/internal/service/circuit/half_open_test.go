package circuit

import (
	"testing"
	"time"
)

func TestChannelKeyBreakerHalfOpenAllowsSingleProbe(t *testing.T) {
	breaker := &ChannelKeyBreaker{
		state:            KeyStateCooling,
		failureCount:     3,
		lastFailure:      time.Now().Add(-time.Minute),
		failureThreshold: 3,
		coolingDuration:  time.Millisecond,
	}

	if !breaker.IsAvailable() {
		t.Fatal("cooling breaker should become probe-eligible after cooling window")
	}
	ok, acquired := breaker.TryAcquireProbe()
	if !ok || !acquired {
		t.Fatalf("expected first probe to be acquired, ok=%v acquired=%v", ok, acquired)
	}
	if breaker.IsAvailable() {
		t.Fatal("half-open breaker must reject concurrent probes")
	}

	breaker.RecordSoftFailure()
	breaker.lastFailure = time.Now().Add(-10 * time.Minute)
	if !breaker.IsAvailable() {
		t.Fatal("after failed probe and elapsed cooling, breaker should be probe-eligible again")
	}

	ok, acquired = breaker.TryAcquireProbe()
	if !ok || !acquired {
		t.Fatalf("expected second probe to be acquired, ok=%v acquired=%v", ok, acquired)
	}
	breaker.RecordSuccess()
	if !breaker.IsAvailable() {
		t.Fatal("successful half-open probe should restore active state")
	}
}

func TestModelConfigBreakerReleaseUnusedProbe(t *testing.T) {
	breaker := &ChannelModelConfigBreaker{
		state:            ModelConfigStateCooling,
		failureCount:     3,
		lastFailure:      time.Now().Add(-time.Minute),
		failureThreshold: 3,
		coolingDuration:  time.Millisecond,
	}

	ok, acquired := breaker.TryAcquireProbe()
	if !ok || !acquired {
		t.Fatalf("expected probe to be acquired, ok=%v acquired=%v", ok, acquired)
	}
	if breaker.IsAvailable() {
		t.Fatal("half-open probe in flight should not be available")
	}
	breaker.ReleaseProbe()
	if !breaker.IsAvailable() {
		t.Fatal("released unused probe should make the cooled breaker probe-eligible again")
	}
}
