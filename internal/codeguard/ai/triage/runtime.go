package triage

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/safehttp"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type runtimeConfig struct {
	Provider       string
	Model          string
	BaseURL        string
	BaseURLFromEnv bool
	APIKey         string
	Timeout        time.Duration
	MockDecision   string
	MockSummary    string
}

const (
	// Non-streaming reasoning requests do not return headers until generation
	// completes. Give every request a realistic base budget, then add time for
	// each verdict the provider must generate while bounding the total cost.
	triageTimeoutBase         = 60 * time.Second
	triageTimeoutPerCandidate = 3 * time.Second
	triageTimeoutCap          = 180 * time.Second
)

// EffectiveTimeout resolves an explicit timeout or derives a bounded timeout
// from the number of candidates sent in one provider request.
func EffectiveTimeout(candidateCount int, explicit time.Duration) time.Duration {
	if explicit > 0 {
		return explicit
	}
	if candidateCount < 1 {
		candidateCount = 1
	}
	timeout := triageTimeoutBase + time.Duration(candidateCount-1)*triageTimeoutPerCandidate
	if timeout > triageTimeoutCap {
		return triageTimeoutCap
	}
	return timeout
}

func discoverRuntime(cfg core.AIConfig, opts core.ScanOptions) runtimeConfig {
	if !aiEnabled(cfg, opts) || (cfg.HybridTriage.Enabled != nil && !*cfg.HybridTriage.Enabled) {
		return runtimeConfig{}
	}
	var timeout time.Duration
	if raw := strings.TrimSpace(os.Getenv("CODEGUARD_AI_TRIAGE_TIMEOUT")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			timeout = parsed
		}
	}
	provider := firstNonEmpty(
		os.Getenv("CODEGUARD_AI_TRIAGE_PROVIDER"),
		cfg.Provider.Type,
	)
	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	// Config provider settings only apply when the effective provider matches
	// the configured one; otherwise an env-selected provider would inherit
	// another provider's model, base URL, or key env.
	configProvider := cfg.Provider
	if !strings.EqualFold(strings.TrimSpace(configProvider.Type), normalizedProvider) {
		configProvider = core.AIProviderConfig{Type: configProvider.Type}
	}
	model := firstNonEmpty(
		os.Getenv("CODEGUARD_AI_TRIAGE_MODEL"),
		configProvider.Model,
	)
	envBaseURL := strings.TrimSpace(os.Getenv("CODEGUARD_AI_TRIAGE_BASE_URL"))
	baseURL := firstNonEmpty(
		envBaseURL,
		configProvider.BaseURL,
	)
	apiKey := firstNonEmpty(
		os.Getenv("CODEGUARD_AI_TRIAGE_API_KEY"),
		apiKeyFromConfig(configProvider),
	)
	if normalizedProvider == "anthropic" {
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
		}
		if strings.TrimSpace(model) == "" {
			model = anthropicDefaultModel
		}
	}
	return runtimeConfig{
		Provider:       normalizedProvider,
		Model:          model,
		BaseURL:        baseURL,
		BaseURLFromEnv: envBaseURL != "",
		APIKey:         apiKey,
		Timeout:        timeout,
		MockDecision:   strings.ToLower(strings.TrimSpace(os.Getenv("CODEGUARD_AI_TRIAGE_DECISION"))),
		MockSummary:    strings.TrimSpace(os.Getenv("CODEGUARD_AI_TRIAGE_SUMMARY")),
	}
}

func (cfg runtimeConfig) enabled() bool {
	return cfg.Provider != ""
}

func (cfg runtimeConfig) validate() error {
	if cfg.Provider == "" {
		return nil
	}
	if cfg.Provider != "mock" && cfg.Model == "" {
		return fmt.Errorf("CODEGUARD_AI_TRIAGE_MODEL is required when CODEGUARD_AI_TRIAGE_PROVIDER is set")
	}
	if err := safehttp.ValidateProviderURL(cfg.BaseURL, cfg.BaseURLFromEnv); err != nil {
		return err
	}
	switch cfg.Provider {
	case "mock":
		return nil
	case "openai":
		return nil
	case "anthropic":
		return nil
	default:
		return fmt.Errorf("unsupported CODEGUARD_AI_TRIAGE_PROVIDER %q", cfg.Provider)
	}
}

func (cfg runtimeConfig) displayName() string {
	if cfg.Model == "" {
		return cfg.Provider
	}
	return cfg.Provider + ":" + cfg.Model
}

func aiEnabled(cfg core.AIConfig, opts core.ScanOptions) bool {
	if opts.EnableAI {
		return true
	}
	if cfg.Enabled != nil && *cfg.Enabled {
		return true
	}
	if strings.TrimSpace(os.Getenv("CODEGUARD_AI_TRIAGE_PROVIDER")) != "" {
		return true
	}
	if strings.TrimSpace(cfg.Provider.Command) != "" {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Provider.Type), "command") {
		return strings.TrimSpace(cfg.Provider.Command) != ""
	}
	if key := strings.TrimSpace(apiKeyFromConfig(cfg.Provider)); key != "" {
		return true
	}
	return false
}

func apiKeyFromConfig(cfg core.AIProviderConfig) string {
	keyEnv := strings.TrimSpace(cfg.APIKeyEnv)
	if keyEnv == "" {
		if !strings.EqualFold(strings.TrimSpace(cfg.Type), "anthropic") {
			return ""
		}
		keyEnv = "ANTHROPIC_API_KEY"
	}
	return os.Getenv(keyEnv)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
