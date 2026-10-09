package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRuntimeRegistryRetryPolicy(t *testing.T) {
	for _, tc := range []struct {
		err   error
		retry bool
	}{
		{fmt.Errorf("unexpected status code 408"), true},
		{fmt.Errorf("unexpected HTTP status: 429"), true},
		{fmt.Errorf("StatusCode: 500"), true},
		{fmt.Errorf("GET https://registry.example: 502 Bad Gateway"), true},
		{fmt.Errorf("unexpected status 503"), true},
		{fmt.Errorf("HTTP 504"), true},
		{fmt.Errorf("TOOMANYREQUESTS: rate limit exceeded"), true},
		{fmt.Errorf("dial tcp: connection reset by peer"), true},
		{fmt.Errorf("i/o timeout"), true},
		{fmt.Errorf("unexpected EOF"), true},
		{fmt.Errorf("unauthorized"), false},
		{fmt.Errorf("unexpected status 401"), false},
		{fmt.Errorf("unexpected status 403"), false},
		{fmt.Errorf("unexpected status 404"), false},
		{fmt.Errorf("manifest unknown"), false},
		{fmt.Errorf("unexpected status 501"), false},
		{fmt.Errorf("unexpected status 505"), false},
		{fmt.Errorf("x509: certificate signed by unknown authority"), false},
		{fmt.Errorf("open auth.json: permission denied"), false},
		{fmt.Errorf("exec: skopeo: executable file not found"), false},
		{fmt.Errorf("parse SPDX predicate: invalid JSON"), false},
		{fmt.Errorf("reading manifest registry.example/runtime:429: invalid reference"), false},
		{fmt.Errorf("status 429: %w", context.Canceled), false},
		{fmt.Errorf("status 500: %w", context.DeadlineExceeded), false},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			calls := 0
			var delays []time.Duration
			_, err := retryRuntimeRegistryOperation(context.Background(), "test", func() ([]byte, error) {
				calls++
				return nil, tc.err
			}, func(_ context.Context, delay time.Duration) error {
				delays = append(delays, delay)
				return nil
			})
			if !errors.Is(err, tc.err) {
				t.Fatalf("final error was lost: %v", err)
			}
			if tc.retry {
				if calls != 4 || !reflect.DeepEqual(delays, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}) {
					t.Fatalf("unexpected retry bound/backoff: %d calls, %v", calls, delays)
				}
			} else if calls != 1 || len(delays) != 0 {
				t.Fatalf("permanent error was retried: %d calls, %v", calls, delays)
			}
		})
	}
}

func TestRuntimeRegistryRetryRecoveryAndCancellation(t *testing.T) {
	calls := 0
	data, err := retryRuntimeRegistryOperation(context.Background(), "test", func() ([]byte, error) {
		calls++
		if calls < 3 {
			return nil, fmt.Errorf("unexpected status code 429")
		}
		return []byte("recovered"), nil
	}, func(context.Context, time.Duration) error { return nil })
	if err != nil || string(data) != "recovered" || calls != 3 {
		t.Fatalf("retry did not recover: %q, %v, %d calls", data, err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls = 0
	_, err = retryRuntimeRegistryOperation(ctx, "test", func() ([]byte, error) {
		calls++
		return nil, fmt.Errorf("unexpected status 500")
	}, func(ctx context.Context, delay time.Duration) error {
		cancel()
		return waitRuntimeRegistryRetry(ctx, delay)
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("backoff ignored cancellation: %v, %d calls", err, calls)
	}
	_, err = retryRuntimeRegistryOperation(ctx, "test", func() ([]byte, error) {
		t.Fatal("canceled operation was executed")
		return nil, nil
	}, waitRuntimeRegistryRetry)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context was ignored: %v", err)
	}
}

func TestRuntimeRegistryCommandsRetry(t *testing.T) {
	for _, tool := range []string{"skopeo", "cosign"} {
		t.Run(tool, func(t *testing.T) {
			root := t.TempDir()
			counter := filepath.Join(root, "calls")
			script := `#!/bin/sh
if [ ! -e "$REGISTRY_RETRY_TEST_COUNTER" ]; then
  printf '%s' first > "$REGISTRY_RETRY_TEST_COUNTER"
  printf '%s' 'unexpected status code 429' >&2
  exit 1
fi
printf '%s' recovered
`
			if err := os.WriteFile(filepath.Join(root, tool), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", root+":"+os.Getenv("PATH"))
			t.Setenv("REGISTRY_RETRY_TEST_COUNTER", counter)
			var data []byte
			var err error
			if tool == "skopeo" {
				data, err = runRuntimeSkopeo(context.Background(), "", "inspect", "--raw", "docker://registry.redhat.io/rhoai/runtime:3.6")
			} else {
				data, err = downloadRuntimeAttestations(context.Background(), "", "registry.redhat.io/rhaii/runtime@sha256:"+strings.Repeat("a", 64))
			}
			if err != nil || string(data) != "recovered" {
				t.Fatalf("command did not recover: %q, %v", data, err)
			}
		})
	}
}
