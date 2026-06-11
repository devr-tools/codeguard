package prompts

import (
	"context"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	secretInterpolationRegex = regexp.MustCompile(`(\$\{[A-Z0-9_]+\}|{{\s*[^}]*secret[^}]*}})`)
	unsafePromptPatterns     = []*regexp.Regexp{
		regexp.MustCompile(`(?i)ignore previous instructions`),
		regexp.MustCompile(`(?i)reveal the system prompt`),
		regexp.MustCompile(`(?i)disregard all prior instructions`),
	}
)

func Run(_ context.Context, env support.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
		findings = append(findings, env.ScanTargetFiles(target, "prompts", env.IsPromptFile, func(file string, data []byte) []core.Finding {
			return findingsForFile(env, file, data)
		})...)
	}
	return env.FinalizeSection("prompts", "AI Prompts", findings)
}

func findingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	for idx, line := range strings.Split(string(data), "\n") {
		if *env.Config.Checks.PromptRules.ForbidSecretInterpolation && secretInterpolationRegex.MatchString(line) {
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "prompts.secret-interpolation", Level: "fail", Path: file, Line: idx + 1, Column: 1, Message: "prompt contains secret interpolation pattern"}))
		}
		if !*env.Config.Checks.PromptRules.ForbidUnsafeInstructions {
			continue
		}
		for _, pattern := range unsafePromptPatterns {
			if pattern.MatchString(line) {
				findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "prompts.unsafe-instructions", Level: "warn", Path: file, Line: idx + 1, Column: 1, Message: "prompt contains unsafe instruction pattern"}))
				break
			}
		}
	}
	return findings
}
