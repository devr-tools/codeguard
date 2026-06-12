package triage

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type runtimeConfig struct {
	Provider     string
	Model        string
	BaseURL      string
	APIKey       string
	Timeout      time.Duration
	MockDecision string
	MockSummary  string
}

func discoverRuntime(cfg core.AIConfig, opts core.ScanOptions) runtimeConfig {
	if !aiEnabled(cfg, opts) || (cfg.HybridTriage.Enabled != nil && !*cfg.HybridTriage.Enabled) {
		return runtimeConfig{}
	}
	timeout := 20 * time.Second
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
	baseURL := firstNonEmpty(
		os.Getenv("CODEGUARD_AI_TRIAGE_BASE_URL"),
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
		Provider:     normalizedProvider,
		Model:        model,
		BaseURL:      baseURL,
		APIKey:       apiKey,
		Timeout:      timeout,
		MockDecision: strings.ToLower(strings.TrimSpace(os.Getenv("CODEGUARD_AI_TRIAGE_DECISION"))),
		MockSummary:  strings.TrimSpace(os.Getenv("CODEGUARD_AI_TRIAGE_SUMMARY")),
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
