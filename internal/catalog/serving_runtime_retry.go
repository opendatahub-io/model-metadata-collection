package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

const runtimeRegistryAttempts = 4

var runtimeRetryableStatus = regexp.MustCompile(`(?i)\b(?:http(?:\s+status)?|status(?:\s*code)?|response(?:\s+status)?)\s*[:=]?\s*(408|429|500|502|503|504)\b`)

// CLI tools expose registry failures in stderr rather than typed HTTP errors.
// Only recognized transient failures are retried; cancellation and permanent
// failures retain the generator's existing strict/skip behavior.
func runtimeRegistryErrorRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, permanent := range []string{"unauthorized", "authentication required", "permission denied", "forbidden", "x509:", "tls: bad certificate"} {
		if strings.Contains(message, permanent) {
			return false
		}
	}
	if runtimeImageMissing(err) {
		return false
	}
	if runtimeRetryableStatus.MatchString(message) {
		return true
	}
	for _, transient := range []string{
		"408 request timeout", "429 too many requests", "toomanyrequests", "too many requests",
		"500 internal server error", "502 bad gateway", "503 service unavailable", "504 gateway timeout",
		"connection reset by peer", "connection refused", "i/o timeout", "tls handshake timeout",
		"temporary failure in name resolution", "temporary network error", "unexpected eof", "net/http: timeout",
	} {
		if strings.Contains(message, transient) {
			return true
		}
	}
	return false
}

func retryRuntimeRegistryOperation(ctx context.Context, name string, operation func() ([]byte, error), wait func(context.Context, time.Duration) error) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := operation()
		if err == nil || attempt == runtimeRegistryAttempts-1 || !runtimeRegistryErrorRetryable(err) {
			return data, err
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		delay := time.Second << attempt
		fmt.Fprintf(os.Stderr, "Retrying %s in %s (%d/%d): %v\n", name, delay, attempt+1, runtimeRegistryAttempts-1, err)
		if err := wait(ctx, delay); err != nil {
			return nil, err
		}
	}
}

func waitRuntimeRegistryRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
