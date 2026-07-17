package quality

import (
	"context"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func cppToolingFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	if !isExplicitCPPTarget(target.Language) {
		return nil
	}
	cfg := env.Config.Checks.QualityRules.CPPTooling
	if !modeEnabled(cfg.ClangFormatMode) && !modeEnabled(cfg.CompilerMode) {
		return nil
	}
	files := make([]string, 0)
	env.VisitTargetFiles(target, func(rel string) bool { return support.IsCPPPath(rel, true) }, func(rel string, _ []byte) {
		files = append(files, rel)
	})
	if len(files) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0)
	if modeEnabled(cfg.ClangFormatMode) {
		result := support.CPPToolResult{Unavailable: true, Err: fmt.Errorf("clang-format runner is unavailable")}
		if env.RunCPPFormat != nil {
			result = env.RunCPPFormat(ctx, target.Path, cfg, files)
		}
		findings = append(findings, cppToolFindings(env, "quality.cpp.clang-format", cfg.ClangFormatMode, result)...)
	}
	if modeEnabled(cfg.CompilerMode) {
		result := support.CPPToolResult{Unavailable: true, Err: fmt.Errorf("c++ compiler runner is unavailable")}
		if env.RunCPPSyntax != nil {
			result = env.RunCPPSyntax(ctx, target.Path, cfg)
		}
		findings = append(findings, cppToolFindings(env, "quality.cpp.compiler-parse", cfg.CompilerMode, result)...)
	}
	return findings
}

func cppToolFindings(env support.Context, ruleID, mode string, result support.CPPToolResult) []core.Finding {
	findings := make([]core.Finding, 0, len(result.Issues)+1)
	for _, issue := range result.Issues {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID: ruleID, Level: "fail", Path: issue.Path, Line: 1, Column: 1, Message: issue.Message,
		}))
	}
	if result.Err == nil || (result.Unavailable && normalizedToolMode(mode) == core.ExternalToolModeAuto) {
		return findings
	}
	level := "warn"
	if normalizedToolMode(mode) == core.ExternalToolModeRequired {
		level = "fail"
	}
	return append(findings, env.NewFinding(support.FindingInput{
		RuleID: ruleID, Level: level, Message: result.Err.Error(),
	}))
}

func isExplicitCPPTarget(language string) bool {
	switch support.NormalizedLanguage(language) {
	case "c++", "cpp", "cxx", "cc":
		return true
	default:
		return false
	}
}

func modeEnabled(mode string) bool {
	mode = normalizedToolMode(mode)
	return mode == core.ExternalToolModeAuto || mode == core.ExternalToolModeRequired
}

func normalizedToolMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}
