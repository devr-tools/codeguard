package quality

import (
	"context"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/semantic"
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func semanticFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	if env.Mode != core.ScanModeDiff {
		return nil
	}
	if !aiProvenanceActive(env) {
		return nil
	}
	findings, err := semantic.Analyze(ctx, semantic.Options{
		Target:    target,
		Language:  support.NormalizedLanguage(target.Language),
		BaseRef:   env.BaseRef,
		DiffText:  env.DiffText,
		CachePath: semanticCachePath(env.Config.Cache),
		Command:   semanticCommand(env.Config.AI),
		Enabled:   semanticEnabled(env),
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
		return nil
	}
	return findings
}

func semanticEnabled(env support.Context) bool {
	if env.Config.AI.Semantic.Enabled != nil {
		return *env.Config.AI.Semantic.Enabled && aiRuntimeEnabled(env)
	}
	return aiRuntimeEnabled(env) && semantic.Enabled()
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
