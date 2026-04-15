package outbox

import (
	"errors"
	"testing"
	"time"
)

func TestApplyWithRetryDropsAfterAttempts(t *testing.T) {
	prevStart := retryBackoffStart
	prevMax := retryBackoffMax
	prevSleep := retrySleep
	prevTimer := retryTimer
	t.Cleanup(func() {
		retryBackoffStart = prevStart
		retryBackoffMax = prevMax
		retrySleep = prevSleep
		retryTimer = prevTimer
	})
	resetBackoffForTest()

	applyCalls := 0
	finalizeCalls := 0
	applyErr := errors.New("apply failed")

	err := applyWithRetry(42, 99, nil, func() error {
		applyCalls++
		return applyErr
	}, func() error {
		finalizeCalls++
		return nil
	}, "test", nil)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if applyCalls != retryAttempts {
		t.Fatalf("expected %d attempts, got %d", retryAttempts, applyCalls)
	}
	if finalizeCalls != 1 {
		t.Fatalf("expected finalize once, got %d", finalizeCalls)
	}
}

func TestApplyWithRetrySucceeds(t *testing.T) {
	prevStart := retryBackoffStart
	prevMax := retryBackoffMax
	prevSleep := retrySleep
	prevTimer := retryTimer
	t.Cleanup(func() {
		retryBackoffStart = prevStart
		retryBackoffMax = prevMax
		retrySleep = prevSleep
		retryTimer = prevTimer
	})
	resetBackoffForTest()

	applyCalls := 0
	finalizeCalls := 0

	err := applyWithRetry(7, 11, nil, func() error {
		applyCalls++
		if applyCalls < 4 {
			return errors.New("apply failed")
		}
		return nil
	}, func() error {
		finalizeCalls++
		return nil
	}, "test", nil)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if applyCalls != 4 {
		t.Fatalf("expected 4 attempts, got %d", applyCalls)
	}
	if finalizeCalls != 1 {
		t.Fatalf("expected finalize once, got %d", finalizeCalls)
	}
}

func resetBackoffForTest() {
	retryBackoffStart = 0
	retryBackoffMax = 0
	retrySleep = func(time.Duration) {}
	retryTimer = func(time.Duration) *time.Timer {
		t := time.NewTimer(0)
		return t
	}
}
