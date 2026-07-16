package codeguard_test

import (
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/httpretry"
)

func fastRetryConfig(maxRetries int) httpretry.Config {
	return httpretry.Config{
		MaxRetries: maxRetries,
		BaseDelay:  time.Millisecond,
		MaxDelay:   5 * time.Millisecond,
	}
}

func buildGet(t *testing.T, url string, builds *atomic.Int64) func() (*http.Request, error) {
	t.Helper()
	return func() (*http.Request, error) {
		if builds != nil {
			builds.Add(1)
		}
		return http.NewRequest(http.MethodGet, url, nil)
	}
}
