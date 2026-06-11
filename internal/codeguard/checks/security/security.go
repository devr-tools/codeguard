package security

import (
	"context"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Run(ctx context.Context, env support.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
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
	}
	return env.FinalizeSection("security", "Security", findings)
}

func commandFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	checks := env.Config.Checks.SecurityRules.LanguageCommands[normalizedLanguage(target.Language)]
	findings := make([]core.Finding, 0, len(checks))
	for _, check := range checks {
		output, err := env.RunCommandCheck(ctx, target.Path, check)
		if err == nil {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "security.command-check",
			Level:   "fail",
			Message: commandFailureMessage(target, check, output, err),
		}))
	}
	return findings
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

func commandFailureMessage(target core.TargetConfig, check core.CommandCheckConfig, output string, err error) string {
	message := fmt.Sprintf("target %q security command %q failed", target.Name, check.Name)
	output = trimmedOutput(output)
	if output != "" {
		message += ": " + output
	} else if err != nil {
		message += ": " + err.Error()
	}
	return message
}

func isGoTarget(target core.TargetConfig) bool {
	language := normalizedLanguage(target.Language)
	return language == "" || language == "go"
}

func isTypeScriptTarget(target core.TargetConfig) bool {
	switch normalizedLanguage(target.Language) {
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return true
	default:
		return false
	}
}

func normalizedLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}

func trimmedOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	output = strings.Join(strings.Fields(output), " ")
	if len(output) > 240 {
		return output[:237] + "..."
	}
	return output
}
