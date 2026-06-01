package retry

import (
	"context"
	"time"
)

// Do executes fn with exponential backoff up to maxAttempts.
func Do(ctx context.Context, maxAttempts int, baseDelay time.Duration, fn func() error) error {
	var err error
	delay := baseDelay
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if attempt == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			delay *= 2
		}
	}
	return err
}
