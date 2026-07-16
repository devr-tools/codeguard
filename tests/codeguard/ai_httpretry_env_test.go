package codeguard_test

import (
	"testing"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/httpretry"
)

func TestFromEnvDefaults(t *testing.T) {
	t.Setenv("CODEGUARD_AI_MAX_RETRIES", "")
	t.Setenv("CODEGUARD_AI_RETRY_BASE_DELAY", "")

	cfg := httpretry.FromEnv()
	if cfg.MaxRetries != 3 {
		t.Fatalf("MaxRetries = %d, want default 3", cfg.MaxRetries)
	}
	if cfg.BaseDelay != 250*time.Millisecond {
		t.Fatalf("BaseDelay = %v, want default 250ms", cfg.BaseDelay)
	}
	if cfg.MaxDelay != 8*time.Second {
		t.Fatalf("MaxDelay = %v, want default 8s", cfg.MaxDelay)
	}
}

func TestFromEnvOverrides(t *testing.T) {
	t.Setenv("CODEGUARD_AI_MAX_RETRIES", " 7 ")
	t.Setenv("CODEGUARD_AI_RETRY_BASE_DELAY", "50ms")

	cfg := httpretry.FromEnv()
	if cfg.MaxRetries != 7 {
		t.Fatalf("MaxRetries = %d, want 7 from env", cfg.MaxRetries)
	}
	if cfg.BaseDelay != 50*time.Millisecond {
		t.Fatalf("BaseDelay = %v, want 50ms from env", cfg.BaseDelay)
	}
}

func TestFromEnvIgnoresInvalidValues(t *testing.T) {
	cases := []struct {
		name       string
		maxRetries string
		baseDelay  string
	}{
		{name: "garbage", maxRetries: "not-a-number", baseDelay: "not-a-duration"},
		{name: "negative retries", maxRetries: "-2", baseDelay: "-10ms"},
		{name: "zero delay", maxRetries: "3", baseDelay: "0s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CODEGUARD_AI_MAX_RETRIES", tc.maxRetries)
			t.Setenv("CODEGUARD_AI_RETRY_BASE_DELAY", tc.baseDelay)

			cfg := httpretry.FromEnv()
			if cfg.BaseDelay != 250*time.Millisecond {
				t.Fatalf("BaseDelay = %v, want default kept for invalid env", cfg.BaseDelay)
			}
			if tc.maxRetries == "3" {
				if cfg.MaxRetries != 3 {
					t.Fatalf("MaxRetries = %d, want 3", cfg.MaxRetries)
				}
			} else if cfg.MaxRetries != 3 {
				t.Fatalf("MaxRetries = %d, want default kept for invalid env", cfg.MaxRetries)
			}
		})
	}
}
