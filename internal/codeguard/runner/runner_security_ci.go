package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	secretPattern            = regexp.MustCompile(`(?i)(secret|token|api[_-]?key|password)\s*[:=]\s*["'][^"']{8,}["']`)
	privateKeyPattern        = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)
	secretInterpolationRegex = regexp.MustCompile(`(\$\{[A-Z0-9_]+\}|{{\s*[^}]*secret[^}]*}})`)
	unsafePromptPatterns     = []*regexp.Regexp{
		regexp.MustCompile(`(?i)ignore previous instructions`),
		regexp.MustCompile(`(?i)reveal the system prompt`),
		regexp.MustCompile(`(?i)disregard all prior instructions`),
	}
)

func (sc scanContext) runSecurity(ctx context.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range sc.cfg.Targets {
		findings = append(findings, scanTargetFiles(sc, target, "security", func(string) bool { return true }, func(file string, data []byte) []core.Finding {
			return securityFindingsForFile(sc, file, data)
		})...)

		mode := strings.ToLower(strings.TrimSpace(sc.cfg.Checks.SecurityRules.GovulncheckMode))
		switch mode {
		case "", "off":
		case "auto", "required":
			govulnFindings, err := runGovulncheck(ctx, target.Path, sc.cfg.Checks.SecurityRules.GovulncheckCommand, sc)
			if err != nil {
				level := "warn"
				if mode == "required" {
					level = "fail"
				}
				findings = append(findings, newFinding(sc, findingInput{
					ruleID:  "security.govulncheck",
					level:   level,
					message: err.Error(),
				}))
			}
			findings = append(findings, govulnFindings...)
		default:
			findings = append(findings, newFinding(sc, findingInput{
				ruleID:  "security.govulncheck",
				level:   "fail",
				message: "govulncheck_mode must be off, auto, or required",
			}))
		}
	}
	return finalizeSection(sc, "security", "Security", findings)
}

func securityFindingsForFile(sc scanContext, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	for idx, line := range strings.Split(string(data), "\n") {
		switch {
		case secretPattern.MatchString(line):
			findings = append(findings, newFinding(sc, findingInput{ruleID: "security.hardcoded-secret", level: "fail", path: file, line: idx + 1, column: 1, message: "possible hardcoded secret detected"}))
		case privateKeyPattern.MatchString(line):
			findings = append(findings, newFinding(sc, findingInput{ruleID: "security.private-key", level: "fail", path: file, line: idx + 1, column: 1, message: "private key material detected"}))
		case strings.Contains(line, "InsecureSkipVerify: true"):
			findings = append(findings, newFinding(sc, findingInput{ruleID: "security.insecure-tls", level: "fail", path: file, line: idx + 1, column: 1, message: "InsecureSkipVerify is enabled"}))
		case strings.Contains(line, "exec.Command(") || strings.Contains(line, "os/exec"):
			findings = append(findings, newFinding(sc, findingInput{ruleID: "security.shell-execution", level: "warn", path: file, line: idx + 1, column: 1, message: "shell execution primitive should be reviewed"}))
		}
	}
	return findings
}

func (sc scanContext) runPrompts(_ context.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range sc.cfg.Targets {
		findings = append(findings, scanTargetFiles(sc, target, "prompts", func(rel string) bool {
			return isPromptFile(sc, rel)
		}, func(file string, data []byte) []core.Finding {
			return promptFindingsForFile(sc, file, data)
		})...)
	}
	return finalizeSection(sc, "prompts", "AI Prompts", findings)
}

func promptFindingsForFile(sc scanContext, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	for idx, line := range strings.Split(string(data), "\n") {
		if *sc.cfg.Checks.PromptRules.ForbidSecretInterpolation && secretInterpolationRegex.MatchString(line) {
			findings = append(findings, newFinding(sc, findingInput{ruleID: "prompts.secret-interpolation", level: "fail", path: file, line: idx + 1, column: 1, message: "prompt contains secret interpolation pattern"}))
		}
		if !*sc.cfg.Checks.PromptRules.ForbidUnsafeInstructions {
			continue
		}
		for _, pattern := range unsafePromptPatterns {
			if pattern.MatchString(line) {
				findings = append(findings, newFinding(sc, findingInput{ruleID: "prompts.unsafe-instructions", level: "warn", path: file, line: idx + 1, column: 1, message: "prompt contains unsafe instruction pattern"}))
				break
			}
		}
	}
	return findings
}

func (sc scanContext) runCI(_ context.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range sc.cfg.Targets {
		findings = append(findings, ciFindingsForTarget(sc, target)...)
	}
	return finalizeSection(sc, "ci", "CI/CD", findings)
}

func ciFindingsForTarget(sc scanContext, target core.TargetConfig) []core.Finding {
	findings := make([]core.Finding, 0)
	if *sc.cfg.Checks.CIRules.RequireWorkflowDir {
		workflowDir := filepath.Join(target.Path, ".github", "workflows")
		if _, err := os.Stat(workflowDir); err != nil {
			findings = append(findings, newFinding(sc, findingInput{ruleID: "ci.required-workflow-dir", level: "fail", path: ".github/workflows", message: "workflow directory is required"}))
		}
	}
	for _, required := range sc.cfg.Checks.CIRules.RequiredWorkflowFiles {
		if _, err := os.Stat(filepath.Join(target.Path, filepath.FromSlash(required))); err != nil {
			findings = append(findings, newFinding(sc, findingInput{ruleID: "ci.required-file", level: "fail", path: required, message: "required workflow file is missing"}))
		}
	}
	for _, required := range sc.cfg.Checks.CIRules.RequiredReleaseFiles {
		if _, err := os.Stat(filepath.Join(target.Path, filepath.FromSlash(required))); err != nil {
			findings = append(findings, newFinding(sc, findingInput{ruleID: "ci.required-file", level: "fail", path: required, message: "required release file is missing"}))
		}
	}
	for _, required := range sc.cfg.Checks.CIRules.RequiredAutomationPaths {
		if _, err := os.Stat(filepath.Join(target.Path, filepath.FromSlash(required))); err != nil {
			findings = append(findings, newFinding(sc, findingInput{ruleID: "ci.required-file", level: "fail", path: required, message: "required automation path is missing"}))
		}
	}
	for _, rule := range sc.cfg.Checks.CIRules.WorkflowContentRules {
		data, err := os.ReadFile(filepath.Join(target.Path, filepath.FromSlash(rule.Path)))
		if err != nil {
			continue
		}
		text := string(data)
		for _, marker := range rule.RequiredContains {
			if !strings.Contains(text, marker) {
				findings = append(findings, newFinding(sc, findingInput{
					ruleID:  "ci.workflow-content",
					level:   "fail",
					path:    rule.Path,
					message: fmt.Sprintf("required workflow marker %q is missing", marker),
				}))
			}
		}
	}
	return findings
}

func (sc scanContext) runCustomRules(_ context.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range sc.cfg.Targets {
		findings = append(findings, scanTargetFiles(sc, target, "custom", func(string) bool { return true }, func(file string, data []byte) []core.Finding {
			localFindings := make([]core.Finding, 0)
			lines := strings.Split(string(data), "\n")
			for _, rule := range sc.customRules {
				if !rule.matchesPath(file) {
					continue
				}
				if rule.contentRegex == nil {
					localFindings = append(localFindings, newFinding(sc, findingInput{
						ruleID:  rule.rule.ID,
						level:   normalizedSeverity(rule.rule.Severity),
						path:    file,
						message: rule.rule.Message,
					}))
					continue
				}
				for idx, line := range lines {
					if rule.contentRegex.MatchString(line) {
						localFindings = append(localFindings, newFinding(sc, findingInput{
							ruleID:  rule.rule.ID,
							level:   normalizedSeverity(rule.rule.Severity),
							path:    file,
							line:    idx + 1,
							column:  1,
							message: rule.rule.Message,
						}))
					}
				}
			}
			return localFindings
		})...)
	}
	return finalizeSection(sc, "custom", "Custom Rules", findings)
}
