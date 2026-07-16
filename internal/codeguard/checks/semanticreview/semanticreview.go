// Package semanticreview centralizes how check sections invoke the
// command-backed semantic review runtime (internal/codeguard/ai/semantic).
//
// The quality and performance sections share one combined request: both build
// byte-identical semantic.Options here (all lenses in one payload), so the
// verdict cache and the in-process single-flight in the semantic package
// collapse them into a single runtime call per scan, and each section
// demultiplexes the response by rule-id prefix via Options.EmitRule. Keep
// every input that feeds the request hash in this package — if sections
// computed options independently, a drifting field would silently double the
// LLM calls.
//
// This lives in its own package (not checks/support) because ai/semantic
// imports runner/support, which imports checks/support; adding the reverse
// edge would create an import cycle.
package semanticreview

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/semantic"
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Enabled reports whether the semantic review runtime is enabled for this
// scan: an explicit ai.semantic.enabled config wins; otherwise the
// CODEGUARD_SEMANTIC_CHECKS env gate applies. Either way the AI runtime
// itself must be on.
func Enabled(env support.Context) bool {
	if env.Config.AI.Semantic.Enabled != nil {
		return *env.Config.AI.Semantic.Enabled && aiRuntimeEnabled(env)
	}
	return aiRuntimeEnabled(env) && semantic.Enabled()
}

func aiRuntimeEnabled(env support.Context) bool {
	return env.AIEnabled || semantic.Enabled()
}

// Options builds the semantic.Analyze options for one section. rulePrefix
// scopes emission ("quality." for the quality section, "performance." for the
// performance section); everything else is identical across callers by
// construction.
func Options(env support.Context, target core.TargetConfig, rulePrefix string) semantic.Options {
	return semantic.Options{
		Target:         target,
		Language:       support.NormalizedLanguage(target.Language),
		BaseRef:        env.BaseRef,
		DiffText:       env.DiffText,
		CachePath:      cachePath(env.Config.Cache),
		Command:        Command(env.Config.AI),
		Enabled:        Enabled(env),
		CheckSelection: selection(env),
		EmitRule: func(ruleID string) bool {
			return strings.HasPrefix(ruleID, rulePrefix)
		},
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
	}
}

// Command resolves the runtime command: an ai.provider command wins, else the
// CODEGUARD_SEMANTIC_COMMAND env value.
func Command(cfg core.AIConfig) string {
	if strings.TrimSpace(cfg.Provider.Type) == "command" && strings.TrimSpace(cfg.Provider.Command) != "" {
		return strings.TrimSpace(strings.Join(append([]string{cfg.Provider.Command}, cfg.Provider.Args...), " "))
	}
	return semantic.Command()
}

// selection returns every lens active for this scan. The performance lens
// rides along only when checks.performance is enabled, which keeps requests
// (and therefore cache keys) byte-identical to previous releases when the
// performance section is off.
func selection(env support.Context) semantic.CheckSelection {
	cfg := env.Config.AI.Semantic
	return semantic.CheckSelection{
		FunctionContract:        cfg.FunctionContract == nil || *cfg.FunctionContract,
		ContractDrift:           cfg.ContractDrift == nil || *cfg.ContractDrift,
		MisleadingErrorMessages: cfg.MisleadingErrorMessages == nil || *cfg.MisleadingErrorMessages,
		TestBehaviorCoverage:    cfg.TestBehaviorCoverage == nil || *cfg.TestBehaviorCoverage,
		TestAdequacy:            cfg.TestAdequacy == nil || *cfg.TestAdequacy,
		PerformanceReview:       env.Config.Checks.Performance != nil && *env.Config.Checks.Performance,
	}
}

func cachePath(cfg core.CacheConfig) string {
	if cfg.Enabled != nil && !*cfg.Enabled {
		return ""
	}
	return semantic.CachePathForBase(cfg.Path)
}
