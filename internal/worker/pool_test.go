package worker

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRunProcessesAllJobs(t *testing.T) {
	var count int32
	jobs := []Job{
		func(context.Context) error {
			atomic.AddInt32(&count, 1)
			return nil
		},
		func(context.Context) error {
			atomic.AddInt32(&count, 1)
			return nil
		},
		func(context.Context) error {
			atomic.AddInt32(&count, 1)
			return nil
		},
	}

	if err := Run(context.Background(), 2, jobs); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&count); got != 3 {
		t.Fatalf("expected all jobs to run, got %d", got)
	}
}

func TestRunCollectsErrors(t *testing.T) {
	errA := errors.New("job a failed")
	errB := errors.New("job b failed")

	jobs := []Job{
		func(context.Context) error { return errA },
		func(context.Context) error { return nil },
		func(context.Context) error { return errB },
	}

	err := Run(context.Background(), 2, jobs)
	if err == nil {
		t.Fatalf("expected joined error")
	}
	text := err.Error()
	if !strings.Contains(text, errA.Error()) || !strings.Contains(text, errB.Error()) {
		t.Fatalf("joined error should include both errors, got: %v", err)
	}
}

func TestRunReturnsContextErrorWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	jobs := []Job{
		func(context.Context) error { return nil },
	}

	err := Run(ctx, 1, jobs)
	if err == nil {
		t.Fatalf("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
