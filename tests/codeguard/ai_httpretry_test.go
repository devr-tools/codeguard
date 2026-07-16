package codeguard_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/httpretry"
)

func TestDoReturnsFirstSuccessWithoutRetry(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := httpretry.Do(context.Background(), server.Client(), fastRetryConfig(3), buildGet(t, server.URL, nil))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 request, got %d", got)
	}
}

func TestDoDoesNotRetryNonRetryableStatus(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	resp, err := httpretry.Do(context.Background(), server.Client(), fastRetryConfig(3), buildGet(t, server.URL, nil))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 surfaced unchanged", resp.StatusCode)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected no retries for 400, got %d requests", got)
	}
}

func TestDoRetriesRetryableStatusesUntilSuccess(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls, builds atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) <= 2 {
					w.WriteHeader(status)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			resp, err := httpretry.Do(context.Background(), server.Client(), fastRetryConfig(3), buildGet(t, server.URL, &builds))
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want eventual 200", resp.StatusCode)
			}
			if got := calls.Load(); got != 3 {
				t.Fatalf("expected 3 requests (2 failures then success), got %d", got)
			}
			if got := builds.Load(); got != 3 {
				t.Fatalf("expected a fresh request per attempt (3 builds), got %d", got)
			}
		})
	}
}

func TestDoSurfacesFinalResponseWhenRetriesExhausted(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	resp, err := httpretry.Do(context.Background(), server.Client(), fastRetryConfig(2), buildGet(t, server.URL, nil))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want final 429 returned unchanged", resp.StatusCode)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected initial attempt plus 2 retries, got %d requests", got)
	}
}

func TestDoWithZeroRetriesMakesSingleAttempt(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	resp, err := httpretry.Do(context.Background(), server.Client(), fastRetryConfig(0), buildGet(t, server.URL, nil))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected a single attempt with MaxRetries=0, got %d", got)
	}
}

func TestDoRetriesNetworkErrorAndReturnsItWhenExhausted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	client := server.Client()
	server.Close() // every attempt now fails with a connection error

	var builds atomic.Int64
	resp, err := httpretry.Do(context.Background(), client, fastRetryConfig(2), buildGet(t, server.URL, &builds))
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected network error after retries are exhausted")
	}
	if resp != nil {
		t.Fatalf("resp = %v, want nil alongside network error", resp)
	}
	if got := builds.Load(); got != 3 {
		t.Fatalf("expected 3 attempts against a dead server, got %d builds", got)
	}
}

func TestDoHonorsRetryAfterSeconds(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// BaseDelay is tiny, so a gap of ~1s proves Retry-After won over backoff.
	start := time.Now()
	resp, err := httpretry.Do(context.Background(), server.Client(), fastRetryConfig(1), buildGet(t, server.URL, nil))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Fatalf("elapsed = %v, want >=~1s honoring Retry-After", elapsed)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 requests, got %d", got)
	}
}

func TestDoIgnoresMalformedRetryAfter(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "soon-ish")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	start := time.Now()
	resp, err := httpretry.Do(context.Background(), server.Client(), fastRetryConfig(1), buildGet(t, server.URL, nil))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("elapsed = %v, want fast fallback to backoff for malformed Retry-After", elapsed)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 requests, got %d", got)
	}
}

func TestDoStopsWaitingWhenContextCanceled(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := httpretry.Config{
		MaxRetries: 5,
		BaseDelay:  10 * time.Second,
		MaxDelay:   10 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	resp, err := httpretry.Do(ctx, server.Client(), cfg, buildGet(t, server.URL, nil))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded from backoff wait", err)
	}
	if resp != nil {
		resp.Body.Close()
		t.Fatal("expected nil response when canceled during backoff")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("elapsed = %v, want prompt return on cancellation", elapsed)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 request before cancellation, got %d", got)
	}
}

func TestDoReturnsBuildErrorImmediately(t *testing.T) {
	buildErr := errors.New("bad request template")
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	resp, err := httpretry.Do(context.Background(), server.Client(), fastRetryConfig(3), func() (*http.Request, error) {
		return nil, buildErr
	})
	if !errors.Is(err, buildErr) {
		t.Fatalf("err = %v, want build error surfaced", err)
	}
	if resp != nil {
		resp.Body.Close()
		t.Fatal("expected nil response for build error")
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("expected no requests when build fails, got %d", got)
	}
}

func TestDoWithNilClientUsesDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := httpretry.Do(context.Background(), nil, fastRetryConfig(0), buildGet(t, server.URL, nil))
	if err != nil {
		t.Fatalf("Do with nil client: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
