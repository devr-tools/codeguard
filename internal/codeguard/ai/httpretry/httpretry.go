// Package httpretry provides shared retry and rate-limit handling for the
// HTTP-backed AI providers (OpenAI-compatible and Anthropic, in both the
// runtime and triage packages).
package httpretry

import (
	"context"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	maxRetriesEnv = "CODEGUARD_AI_MAX_RETRIES"
	baseDelayEnv  = "CODEGUARD_AI_RETRY_BASE_DELAY"

	defaultMaxRetries = 3
	defaultBaseDelay  = 250 * time.Millisecond
	defaultMaxDelay   = 8 * time.Second
	maxRetryAfter     = 30 * time.Second

	// drainLimit caps how much of a discarded response body we read before a
	// retry so keep-alive can reuse the connection without unbounded reads.
	drainLimit = 4 << 20
)

// Config controls retry behavior for one logical provider request.
type Config struct {
	// MaxRetries is the number of additional attempts after the first
	// request fails with a retryable status or network error.
	MaxRetries int
	// BaseDelay is the first backoff delay; later attempts double it.
	BaseDelay time.Duration
	// MaxDelay caps the computed exponential backoff.
	MaxDelay time.Duration
}

// FromEnv builds a Config from CODEGUARD_AI_MAX_RETRIES and
// CODEGUARD_AI_RETRY_BASE_DELAY, falling back to safe defaults.
func FromEnv() Config {
	cfg := Config{
		MaxRetries: defaultMaxRetries,
		BaseDelay:  defaultBaseDelay,
		MaxDelay:   defaultMaxDelay,
	}
	if raw := strings.TrimSpace(os.Getenv(maxRetriesEnv)); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			cfg.MaxRetries = parsed
		}
	}
	if raw := strings.TrimSpace(os.Getenv(baseDelayEnv)); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			cfg.BaseDelay = parsed
		}
	}
	return cfg
}

func (cfg Config) normalized() Config {
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = defaultBaseDelay
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = defaultMaxDelay
	}
	return cfg
}

// Do executes build() to create a fresh request for every attempt and retries
// on network errors, 429 responses, and 5xx responses with exponential
// backoff plus jitter. A Retry-After header on a retryable response is
// honored when parseable. The final attempt's response or error is returned
// unchanged so callers keep their existing status handling.
func Do(ctx context.Context, client *http.Client, cfg Config, build func() (*http.Request, error)) (*http.Response, error) {
	cfg = cfg.normalized()
	if client == nil {
		client = http.DefaultClient
	}

	var lastErr error
	for attempt := 0; ; attempt++ {
		req, err := build()
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err == nil && !retryableStatus(resp.StatusCode) {
			return resp, nil
		}
		if attempt >= cfg.MaxRetries {
			// Out of retries: surface the final outcome unchanged.
			return resp, err
		}

		delay := backoffDelay(cfg, attempt)
		if err != nil {
			lastErr = err
		} else {
			if retryAfter, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
				delay = retryAfter
			}
			drainAndClose(resp)
		}
		if waitErr := wait(ctx, delay); waitErr != nil {
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, waitErr
		}
	}
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func backoffDelay(cfg Config, attempt int) time.Duration {
	delay := cfg.BaseDelay << uint(attempt)
	if delay > cfg.MaxDelay || delay <= 0 {
		delay = cfg.MaxDelay
	}
	// Equal jitter: keep half the delay deterministic and randomize the rest
	// so concurrent scans do not retry in lockstep.
	half := delay / 2
	return half + time.Duration(rand.Int63n(int64(half)+1)) //nolint:gosec // retry jitter only, not security-sensitive
}

func parseRetryAfter(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return capRetryAfter(time.Duration(seconds) * time.Second), true
	}
	if at, err := http.ParseTime(value); err == nil {
		until := time.Until(at)
		if until < 0 {
			until = 0
		}
		return capRetryAfter(until), true
	}
	return 0, false
}

func capRetryAfter(delay time.Duration) time.Duration {
	if delay > maxRetryAfter {
		return maxRetryAfter
	}
	return delay
}

// drainAndClose reads and discards the remaining response body (up to a cap)
// before closing it so the underlying connection can be reused on retry.
func drainAndClose(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, drainLimit))
	_ = resp.Body.Close()
}

func wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
