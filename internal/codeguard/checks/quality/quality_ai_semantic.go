package quality

import (
	"context"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/semantic"
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func semanticFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	if !semanticEligible(env) {
		return nil
	}
	command := semanticCommand(env.Config.AI)
	if strings.TrimSpace(command) == "" {
		return []core.Finding{semanticRuntimeFinding(env, target, "semantic review is enabled but no semantic command is configured")}
	}
	findings, err := semantic.Analyze(ctx, semantic.Options{
		Target:         target,
		Language:       support.NormalizedLanguage(target.Language),
		BaseRef:        env.BaseRef,
		DiffText:       env.DiffText,
		CachePath:      semanticCachePath(env.Config.Cache),
		Command:        command,
		Enabled:        semanticEnabled(env),
		CheckSelection: semanticCheckSelection(env.Config.AI.Semantic),
		NewFinding: func(ruleID string, level string, path string, line int, message string) core.Finding {
			return env.NewFinding(support.FindingInput{
				RuleID:  ruleID,
				Level:   level,
				Path:    path,
				Line:    line,
				Column:  1,
				Message: message,
			})
		},
	})
	if err != nil {
		return []core.Finding{semanticRuntimeFinding(env, target, fmt.Sprintf("semantic review command failed for target %q: %v", target.Name, err))}
	}
	return findings
}

func semanticRuntimeFinding(env support.Context, _ core.TargetConfig, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  "quality.ai.semantic-runtime",
		Level:   "fail",
		Path:    "",
		Line:    0,
		Column:  0,
		Message: message,
	})
}

func semanticEligible(env support.Context) bool {
	return semanticEnabled(env)
}

func semanticEnabled(env support.Context) bool {
	if env.Config.AI.Semantic.Enabled != nil {
		return *env.Config.AI.Semantic.Enabled && aiRuntimeEnabled(env)
	}
	return aiRuntimeEnabled(env) && semantic.Enabled()
}

func semanticCheckSelection(cfg core.AISemanticConfig) semantic.CheckSelection {
	return semantic.CheckSelection{
		FunctionContract:        cfg.FunctionContract == nil || *cfg.FunctionContract,
		ContractDrift:           cfg.ContractDrift == nil || *cfg.ContractDrift,
		MisleadingErrorMessages: cfg.MisleadingErrorMessages == nil || *cfg.MisleadingErrorMessages,
		TestBehaviorCoverage:    cfg.TestBehaviorCoverage == nil || *cfg.TestBehaviorCoverage,
		TestAdequacy:            cfg.TestAdequacy == nil || *cfg.TestAdequacy,
	}
}

func semanticCommand(cfg core.AIConfig) string {
	if strings.TrimSpace(cfg.Provider.Type) == "command" && strings.TrimSpace(cfg.Provider.Command) != "" {
		return strings.TrimSpace(strings.Join(append([]string{cfg.Provider.Command}, cfg.Provider.Args...), " "))
	}
	return semantic.Command()
}

func aiRuntimeEnabled(env support.Context) bool {
	return env.AIEnabled || semantic.Enabled()
}

func semanticCachePath(cfg core.CacheConfig) string {
	if cfg.Enabled != nil && !*cfg.Enabled {
		return ""
	}
	return semantic.CachePathForBase(cfg.Path)
}
