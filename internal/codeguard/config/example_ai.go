package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

func exampleAIConfig() core.AIConfig {
	return core.AIConfig{
		Enabled:      boolPtr(false),
		Provider:     exampleAIProviderConfig(),
		Cache:        core.AICacheConfig{Path: ".codeguard/ai-cache.json"},
		HybridTriage: exampleAIHybridTriageConfig(),
		Semantic:     exampleAISemanticConfig(),
		AutoFix:      exampleAIAutoFixConfig(),
	}
}

func exampleAIProviderConfig() core.AIProviderConfig {
	return core.AIProviderConfig{ //nolint:gosec // APIKeyEnv is an env var name, not a credential
		Type:      "openai",
		Model:     "gpt-5",
		BaseURL:   "https://api.openai.com/v1",
		APIKeyEnv: "OPENAI_API_KEY",
	}
}

func exampleAIHybridTriageConfig() core.AIHybridTriageConfig {
	return core.AIHybridTriageConfig{
		Enabled:             boolPtr(true),
		SuppressDismissed:   boolPtr(true),
		CandidateSections:   []string{"Code Quality", "Design Patterns", "Security", "Custom Rules"},
		CandidateSeverities: []string{"warn", "fail"},
	}
}

func exampleAISemanticConfig() core.AISemanticConfig {
	return core.AISemanticConfig{
		Enabled:                 boolPtr(true),
		FunctionContract:        boolPtr(true),
		ContractDrift:           boolPtr(true),
		MisleadingErrorMessages: boolPtr(true),
		TestBehaviorCoverage:    boolPtr(true),
		TestAdequacy:            boolPtr(true),
	}
}

func exampleAIAutoFixConfig() core.AIAutoFixConfig {
	return core.AIAutoFixConfig{
		Enabled:     boolPtr(false),
		VerifyTests: boolPtr(true),
		MaxFixes:    5,
	}
}
