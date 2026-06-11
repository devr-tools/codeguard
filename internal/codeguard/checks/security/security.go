package security

import (
	"context"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	secretPattern     = regexp.MustCompile(`(?i)(secret|token|api[_-]?key|password)\s*[:=]\s*["'][^"']{8,}["']`)
	privateKeyPattern = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)
)

func Run(ctx context.Context, env support.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
		findings = append(findings, env.ScanTargetFiles(target, "security", func(string) bool { return true }, func(file string, data []byte) []core.Finding {
			return findingsForFile(env, file, data)
		})...)

		mode := strings.ToLower(strings.TrimSpace(env.Config.Checks.SecurityRules.GovulncheckMode))
		switch mode {
		case "", "off":
		case "auto", "required":
			govulnFindings, err := env.RunGovulncheck(ctx, target.Path, env.Config.Checks.SecurityRules.GovulncheckCommand)
			if err != nil {
				level := "warn"
				if mode == "required" {
					level = "fail"
				}
				findings = append(findings, env.NewFinding(support.FindingInput{
					RuleID:  "security.govulncheck",
					Level:   level,
					Message: err.Error(),
				}))
			}
			findings = append(findings, govulnFindings...)
		default:
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "security.govulncheck",
				Level:   "fail",
				Message: "govulncheck_mode must be off, auto, or required",
			}))
		}
	}
	return env.FinalizeSection("security", "Security", findings)
}

func findingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	for idx, line := range strings.Split(string(data), "\n") {
		switch {
		case secretPattern.MatchString(line):
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.hardcoded-secret", Level: "fail", Path: file, Line: idx + 1, Column: 1, Message: "possible hardcoded secret detected"}))
		case privateKeyPattern.MatchString(line):
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.private-key", Level: "fail", Path: file, Line: idx + 1, Column: 1, Message: "private key material detected"}))
		case strings.Contains(line, "InsecureSkipVerify: true"):
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.insecure-tls", Level: "fail", Path: file, Line: idx + 1, Column: 1, Message: "InsecureSkipVerify is enabled"}))
		case strings.Contains(line, "exec.Command(") || strings.Contains(line, "os/exec"):
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.shell-execution", Level: "warn", Path: file, Line: idx + 1, Column: 1, Message: "shell execution primitive should be reviewed"}))
		}
	}
	return findings
}
