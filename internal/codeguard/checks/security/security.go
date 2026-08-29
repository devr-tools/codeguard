package security

import (
	"context"
	"strings"
	"sync"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Run is the security section entrypoint; govulncheck only applies to Go
// targets, so non-Go languages rely on configured commands instead.
func Run(ctx context.Context, env support.Context) core.SectionResult {
	var findings []core.Finding
	var diagnostics []core.Diagnostic
	for _, target := range env.Config.Targets {
		result := securityTargetScan(ctx, env, target)
		findings = append(findings, result.findings...)
		diagnostics = append(diagnostics, result.diagnostics...)
	}
	return env.FinalizeSectionWithDiagnostics("security", "Security", findings, diagnostics)
}

type targetScanResult struct {
	findings    []core.Finding
	diagnostics []core.Diagnostic
}

func securityTargetScan(ctx context.Context, env support.Context, target core.TargetConfig) targetScanResult {
	findings := make([]core.Finding, 0)
	diagnostics := make([]core.Diagnostic, 0)

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
		var diagnosticMu sync.Mutex
		findings = append(findings, env.ScanTargetFiles(target, "security-secrets", func(string) bool { return true }, func(file string, data []byte) []core.Finding {
			fileFindings, fileDiagnostics := secretResultsForFile(env, file, data, scanner)
			diagnosticMu.Lock()
			diagnostics = append(diagnostics, fileDiagnostics...)
			diagnosticMu.Unlock()
			return fileFindings
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
		govulnResult := govulncheckFindings(ctx, env, target)
		findings = append(findings, govulnResult.Findings...)
		diagnostics = append(diagnostics, govulnResult.Diagnostics...)
	}
	return targetScanResult{findings: findings, diagnostics: diagnostics}
}

func commandFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return support.SectionCommandFindings(ctx, env, target, support.SectionCommandSpec{
		Checks:  env.Config.Checks.SecurityRules.LanguageCommands[support.NormalizedLanguage(target.Language)],
		RuleID:  "security.command-check",
		Section: "security",
	})
}

func govulncheckFindings(ctx context.Context, env support.Context, target core.TargetConfig) support.GovulncheckResult {
	mode := strings.ToLower(strings.TrimSpace(env.Config.Checks.SecurityRules.GovulncheckMode))
	switch mode {
	case "", "off":
		return support.GovulncheckResult{}
	case "auto", "required":
		result := env.RunGovulncheck(ctx, target.Path, env.Config.Checks.SecurityRules.GovulncheckCommand)
		if mode == "auto" {
			for i := range result.Diagnostics {
				result.Diagnostics[i].Level = "warn"
			}
		}
		return result
	default:
		return support.GovulncheckResult{Diagnostics: []core.Diagnostic{{ID: "scan.govulncheck.config", Level: "fail", Kind: "configuration", Message: "govulncheck_mode must be off, auto, or required", Operational: true}}}
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
