package triage

import (
	"context"
)

type provider interface {
	Triage(ctx context.Context, candidates []candidate) (map[string]providerVerdict, error)
}

func newProvider(cfg runtimeConfig) provider {
	if injected := injectedTestProvider(cfg.Provider); injected != nil {
		return injected
	}
	switch cfg.Provider {
	case "openai":
		return openAIProvider{cfg: cfg}
	case "anthropic":
		return anthropicProvider{cfg: cfg}
	default:
		return noopProvider{}
	}
}

type noopProvider struct{}

func (noopProvider) Triage(_ context.Context, _ []candidate) (map[string]providerVerdict, error) {
	return map[string]providerVerdict{}, nil
}
