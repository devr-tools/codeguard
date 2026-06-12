package triage

import (
	"context"
	"os"
	"strconv"
	"strings"
)

type provider interface {
	Triage(ctx context.Context, candidates []candidate) (map[string]providerVerdict, error)
}

func newProvider(cfg runtimeConfig) provider {
	switch cfg.Provider {
	case "mock":
		return mockProvider{cfg: cfg}
	case "openai":
		return openAIProvider{cfg: cfg}
	case "anthropic":
		return anthropicProvider{cfg: cfg}
	default:
		return noopProvider{}
	}
}

type noopProvider struct{}

func (noopProvider) Triage(ctx context.Context, candidates []candidate) (map[string]providerVerdict, error) {
	return map[string]providerVerdict{}, nil
}

type mockProvider struct {
	cfg runtimeConfig
}

func (provider mockProvider) Triage(ctx context.Context, candidates []candidate) (map[string]providerVerdict, error) {
	verdicts := make(map[string]providerVerdict, len(candidates))
	decision := provider.cfg.MockDecision
	if decision == "" {
		decision = "keep"
	}
	summary := provider.cfg.MockSummary
	for _, item := range candidates {
		verdicts[item.hash] = providerVerdict{
			Decision: decision,
			Summary:  summary,
		}
	}
	incrementMockCountFile()
	return verdicts, nil
}

func incrementMockCountFile() {
	path := strings.TrimSpace(os.Getenv("CODEGUARD_AI_TRIAGE_COUNT_FILE"))
	if path == "" {
		return
	}
	current := 0
	if data, err := os.ReadFile(path); err == nil {
		if parsed, parseErr := strconv.Atoi(strings.TrimSpace(string(data))); parseErr == nil {
			current = parsed
		}
	}
	_ = os.WriteFile(path, []byte(strconv.Itoa(current+1)), 0o644)
}
