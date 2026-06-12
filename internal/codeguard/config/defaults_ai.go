package config

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func applyAIDefaults(dst *core.AIConfig, def core.AIConfig) {
	applyAIProviderDefaults(&dst.Provider, def.Provider)
	applyAIHybridTriageDefaults(&dst.HybridTriage, def.HybridTriage)
	applyAISemanticDefaults(&dst.Semantic, def.Semantic)
	applyAIAutoFixDefaults(&dst.AutoFix, def.AutoFix)
}

func applyAIProviderDefaults(dst *core.AIProviderConfig, def core.AIProviderConfig) {
	if dst.Type == "" {
		dst.Type = def.Type
	}
	// Model, base URL, and key env defaults are provider-flavored; applying
	// them to a different provider type (for example anthropic) would point
	// it at another provider's endpoint and credentials.
	if !strings.EqualFold(strings.TrimSpace(dst.Type), strings.TrimSpace(def.Type)) {
		return
	}
	if dst.Model == "" {
		dst.Model = def.Model
	}
	if dst.BaseURL == "" {
		dst.BaseURL = def.BaseURL
	}
	if dst.APIKeyEnv == "" {
		dst.APIKeyEnv = def.APIKeyEnv
	}
}

func applyAIHybridTriageDefaults(dst *core.AIHybridTriageConfig, def core.AIHybridTriageConfig) {
	if dst.Enabled == nil {
		dst.Enabled = boolPtr(true)
	}
	if dst.SuppressDismissed == nil {
		dst.SuppressDismissed = boolPtr(true)
	}
	if dst.CandidateSections == nil {
		dst.CandidateSections = append([]string(nil), def.CandidateSections...)
	}
	if dst.CandidateSeverities == nil {
		dst.CandidateSeverities = append([]string(nil), def.CandidateSeverities...)
	}
}

func applyAISemanticDefaults(dst *core.AISemanticConfig, def core.AISemanticConfig) {
	if dst.Enabled == nil {
		dst.Enabled = boolPtr(true)
	}
	if dst.FunctionContract == nil {
		dst.FunctionContract = boolPtr(true)
	}
	if dst.MisleadingErrorMessages == nil {
		dst.MisleadingErrorMessages = boolPtr(true)
	}
	if dst.TestBehaviorCoverage == nil {
		dst.TestBehaviorCoverage = boolPtr(true)
	}
}

func applyAIAutoFixDefaults(dst *core.AIAutoFixConfig, def core.AIAutoFixConfig) {
	if dst.Enabled == nil {
		dst.Enabled = boolPtr(false)
	}
	if dst.VerifyTests == nil {
		dst.VerifyTests = boolPtr(true)
	}
	if dst.MaxFixes == 0 {
		dst.MaxFixes = def.MaxFixes
	}
	if dst.TestCommands == nil && len(def.TestCommands) > 0 {
		dst.TestCommands = append([]core.CommandCheckConfig(nil), def.TestCommands...)
	}
}
