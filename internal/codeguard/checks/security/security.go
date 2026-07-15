package security

import (
	"context"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Run is the security section entrypoint; govulncheck only applies to Go
// targets, so non-Go languages rely on configured commands instead.
func Run(ctx context.Context, env support.Context) core.SectionResult {
	return support.RunTargetSection(ctx, env, "security", "Security", securityTargetFindings)
}

func securityTargetFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	findings := make([]core.Finding, 0)

	// Hardcoded secret/credential detection is language-agnostic and runs for
	// every target (including TypeScript/JavaScript, which otherwise bypass
	// findingsForFile). Built once per target so allowlist/custom patterns are
	// compiled a single time.
	// Use a distinct cache section id ("security-secrets") so this pass does not
	// collide with the per-file cache of the language pass below, which also
	// scans the "security" section for the same files.
	scanner, scannerIssues := BuildScanner(env.Config.Checks.SecurityRules.Secrets)
	for _, issue := range scannerIssues {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "security.secrets-config",
			Level:   "fail",
			Message: issue,
		}))
	}
	if scanner.Enabled() {
		findings = append(findings, env.ScanTargetFiles(target, "security-secrets", func(string) bool { return true }, func(file string, data []byte) []core.Finding {
			return secretFindingsForFile(env, file, data, scanner)
		})...)
	}

	// The A09 heuristics (secrets in log calls, raw errors in HTTP responses)
	// need their own repository pass because TypeScript/JavaScript targets
	// bypass findingsForFile whenever the semantic engine claims them, which
	// would silently skip these rules. A distinct cache section id ensures no
	// collision with the per-file caches of the passes below.
	findings = append(findings, env.ScanTargetFiles(target, "security-a09", func(string) bool { return true }, func(file string, data []byte) []core.Finding {
		return a09FindingsForFile(env, file, strings.ReplaceAll(string(data), "\r\n", "\n"))
	})...)

	if isTypeScriptTarget(target) {
		findings = append(findings, typeScriptTargetFindings(ctx, env, target)...)
	} else {
		findings = append(findings, env.ScanTargetFiles(target, "security", func(string) bool { return true }, func(file string, data []byte) []core.Finding {
			return findingsForFile(env, file, data)
		})...)
	}
	findings = append(findings, commandFindings(ctx, env, target)...)

	if isGoTarget(target) {
		findings = append(findings, govulncheckFindings(ctx, env, target)...)
	}
	return findings
}

func commandFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return support.SectionCommandFindings(ctx, env, target, support.SectionCommandSpec{
		Checks:  env.Config.Checks.SecurityRules.LanguageCommands[support.NormalizedLanguage(target.Language)],
		RuleID:  "security.command-check",
		Section: "security",
	})
}

func govulncheckFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	mode := strings.ToLower(strings.TrimSpace(env.Config.Checks.SecurityRules.GovulncheckMode))
	switch mode {
	case "", "off":
		return nil
	case "auto", "required":
		govulnFindings, err := env.RunGovulncheck(ctx, target.Path, env.Config.Checks.SecurityRules.GovulncheckCommand)
		if err == nil {
			return govulnFindings
		}
		level := "warn"
		if mode == "required" {
			level = "fail"
		}
		return append(govulnFindings, env.NewFinding(support.FindingInput{
			RuleID:  "security.govulncheck",
			Level:   level,
			Message: err.Error(),
		}))
	default:
		return []core.Finding{env.NewFinding(support.FindingInput{
			RuleID:  "security.govulncheck",
			Level:   "fail",
			Message: "govulncheck_mode must be off, auto, or required",
		})}
	}
}

func isGoTarget(target core.TargetConfig) bool {
	language := support.NormalizedLanguage(target.Language)
	return language == "" || language == "go"
}

func isTypeScriptTarget(target core.TargetConfig) bool {
	switch support.NormalizedLanguage(target.Language) {
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return true
	default:
		return false
	}
}
