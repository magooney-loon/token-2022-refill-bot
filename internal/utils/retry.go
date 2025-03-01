package utils

import (
	"fmt"
	"time"
)

// WithRetry executes the given function with retries
func WithRetry[T any](fn func() (T, error), maxRetries int, delay time.Duration) (T, error) {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			Debug("Retrying operation",
				"attempt", i,
				"max_retries", maxRetries,
				"delay", delay.String())
			time.Sleep(delay)
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err
		Error("Operation failed, will retry", err,
			"attempt", i,
			"max_retries", maxRetries)
	}

	var zero T
	return zero, fmt.Errorf("operation failed after %d retries: %w", maxRetries, lastErr)
}
