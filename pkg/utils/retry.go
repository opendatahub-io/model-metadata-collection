package utils

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"
)

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
	OverallTimeout time.Duration // Maximum total time for all retries
}

// DefaultRetryConfig provides sensible defaults for registry operations
var DefaultRetryConfig = RetryConfig{
	MaxRetries:     3,
	InitialBackoff: 1 * time.Second,
	MaxBackoff:     30 * time.Second,
	Multiplier:     2.0,
	OverallTimeout: 2 * time.Minute,
}

// RetryWithExponentialBackoff retries a function with exponential backoff
// Returns the result of the function or the last error encountered
func RetryWithExponentialBackoff[T any](config RetryConfig, operation func() (T, error), operationName string) (T, error) {
	var result T
	var err error

	// Create context with overall timeout if configured
	ctx := context.Background()
	var cancel context.CancelFunc
	if config.OverallTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, config.OverallTimeout)
		defer cancel()
	}

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check if context has been cancelled (timeout exceeded)
		select {
		case <-ctx.Done():
			log.Printf("  Retry timeout exceeded for %s: %v", operationName, ctx.Err())
			// Wrap ctx.Err() instead of err to avoid wrapping nil if timeout occurs before first operation
			errToReturn := err
			if errToReturn == nil {
				errToReturn = ctx.Err()
			}
			return result, fmt.Errorf("retry timeout exceeded after %v: %w", config.OverallTimeout, errToReturn)
		default:
			// Continue with retry attempt
		}

		if attempt > 0 {
			// Calculate backoff duration with exponential increase
			backoff := float64(config.InitialBackoff) * math.Pow(config.Multiplier, float64(attempt-1))
			backoffDuration := time.Duration(backoff)

			// Cap at max backoff
			backoffDuration = min(backoffDuration, config.MaxBackoff)

			log.Printf("  Retry %d/%d for %s after %v backoff", attempt, config.MaxRetries, operationName, backoffDuration)

			// Use context-aware sleep
			select {
			case <-time.After(backoffDuration):
				// Sleep completed normally
			case <-ctx.Done():
				log.Printf("  Retry timeout exceeded during backoff for %s: %v", operationName, ctx.Err())
				// Wrap ctx.Err() instead of err to avoid wrapping nil if timeout occurs before first operation
				errToReturn := err
				if errToReturn == nil {
					errToReturn = ctx.Err()
				}
				return result, fmt.Errorf("retry timeout exceeded after %v: %w", config.OverallTimeout, errToReturn)
			}
		}

		result, err = operation()
		if err == nil {
			if attempt > 0 {
				log.Printf("  Successfully recovered after %d retries for %s", attempt, operationName)
			}
			return result, nil
		}

		// Log the error (except on last attempt where we'll return it)
		if attempt < config.MaxRetries {
			log.Printf("  Attempt %d/%d failed for %s: %v", attempt+1, config.MaxRetries+1, operationName, err)
		}
	}

	// All retries exhausted
	log.Printf("  All %d retry attempts exhausted for %s: %v", config.MaxRetries+1, operationName, err)
	return result, err
}
