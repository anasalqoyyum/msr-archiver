package main

import (
	"context"
	"errors"
	"testing"
)

func TestWithRetryEventuallySucceeds(t *testing.T) {
	calls := 0
	err := withRetry(context.Background(), 3, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withRetry should eventually succeed: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
}

func TestWithRetryReturnsLastError(t *testing.T) {
	expected := errors.New("permanent")
	calls := 0
	err := withRetry(context.Background(), 2, func() error {
		calls++
		return expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected last error %v, got %v", expected, err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls)
	}
}

func TestWithRetryRespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := withRetry(ctx, 3, func() error {
		return errors.New("retryable")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestWithRetryResultEventuallySucceeds(t *testing.T) {
	calls := 0
	value, err := withRetryResult(context.Background(), 3, func() (int, error) {
		calls++
		if calls < 2 {
			return 0, errors.New("transient")
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("withRetryResult failed: %v", err)
	}
	if value != 42 {
		t.Fatalf("unexpected value: %d", value)
	}
	if calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls)
	}
}
