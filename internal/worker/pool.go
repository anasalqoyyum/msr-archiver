package worker

import (
	"context"
	"errors"
	"sync"
)

// Job is a unit of work to execute in the pool.
type Job func(context.Context) error

// Run executes jobs with bounded concurrency and returns a joined error.
func Run(ctx context.Context, workers int, jobs []Job) error {
	if workers < 1 {
		workers = 1
	}
	if len(jobs) == 0 {
		return nil
	}

	jobCh := make(chan Job)
	errCh := make(chan error, len(jobs))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				if err := job(ctx); err != nil {
					errCh <- err
				}
			}
		}()
	}

enqueueLoop:
	for _, job := range jobs {
		select {
		case <-ctx.Done():
			break enqueueLoop
		case jobCh <- job:
		}
	}
	close(jobCh)

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		errs = append(errs, ctxErr)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
