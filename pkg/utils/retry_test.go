package utils

import (
	"errors"
	"testing"
	"time"
)

func TestRetryWithExponentialBackoff_Success(t *testing.T) {
	attempts := 0
	config := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     2.0,
	}

	result, err := RetryWithExponentialBackoff(config, func() (string, error) {
		attempts++
		if attempts < 2 {
			return "", errors.New("temporary failure")
		}
		return "success", nil
	}, "test operation")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected result 'success', got: %s", result)
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got: %d", attempts)
	}
}

func TestRetryWithExponentialBackoff_AllFailures(t *testing.T) {
	attempts := 0
	config := RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     2.0,
	}

	_, err := RetryWithExponentialBackoff(config, func() (string, error) {
		attempts++
		return "", errors.New("persistent failure")
	}, "test operation")

	if err == nil {
		t.Error("Expected error, got nil")
	}
	expectedAttempts := config.MaxRetries + 1 // Initial attempt + retries
	if attempts != expectedAttempts {
		t.Errorf("Expected %d attempts, got: %d", expectedAttempts, attempts)
	}
}

func TestRetryWithExponentialBackoff_ImmediateSuccess(t *testing.T) {
	attempts := 0
	config := DefaultRetryConfig

	result, err := RetryWithExponentialBackoff(config, func() (int, error) {
		attempts++
		return 42, nil
	}, "test operation")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result != 42 {
		t.Errorf("Expected result 42, got: %d", result)
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got: %d", attempts)
	}
}

func TestRetryWithExponentialBackoff_BackoffCapping(t *testing.T) {
	config := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     10.0, // High multiplier to test capping
	}

	start := time.Now()
	_, _ = RetryWithExponentialBackoff(config, func() (bool, error) {
		return false, errors.New("always fail")
	}, "test operation")
	elapsed := time.Since(start)

	// With max backoff capping, total time should be around:
	// 50ms + 100ms + 100ms + 100ms + 100ms = 450ms (plus some overhead)
	// Without capping, it would be much higher due to 10x multiplier
	if elapsed > 1*time.Second {
		t.Errorf("Backoff not properly capped, took %v", elapsed)
	}
}

func TestRetryWithExponentialBackoff_TimeoutProtection(t *testing.T) {
	config := RetryConfig{
		MaxRetries:     10,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     500 * time.Millisecond,
		Multiplier:     2.0,
		OverallTimeout: 300 * time.Millisecond, // Timeout before all retries complete
	}

	attempts := 0
	start := time.Now()
	_, err := RetryWithExponentialBackoff(config, func() (string, error) {
		attempts++
		time.Sleep(50 * time.Millisecond) // Simulate slow operation
		return "", errors.New("slow failure")
	}, "test operation")
	elapsed := time.Since(start)

	// Should timeout before all retries complete
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	// Should not complete all retries due to timeout
	if attempts > config.MaxRetries {
		t.Errorf("Completed %d attempts, expected less than %d due to timeout", attempts, config.MaxRetries)
	}

	// Should timeout around the configured timeout duration (with some overhead)
	if elapsed > config.OverallTimeout+200*time.Millisecond {
		t.Errorf("Timeout not enforced properly, took %v (expected ~%v)", elapsed, config.OverallTimeout)
	}
}
