package design

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
		switch normalizedLanguage(target.Language) {
		case "", "go":
			findings = append(findings, goTargetFindings(env, target)...)
		case "typescript", "javascript", "ts", "tsx", "js", "jsx":
			findings = append(findings, typeScriptTargetFindings(env, target)...)
		case "python", "py":
			findings = append(findings, pythonTargetFindings(env, target)...)
		}
		findings = append(findings, commandFindings(ctx, env, target)...)
	}
	return env.FinalizeSection("design", "Design Patterns", findings)
}

func normalizedLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}

func typeScriptTargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	return typeScriptTargetFindingsImpl(env, target)
}

func commandFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	checks := env.Config.Checks.DesignRules.LanguageCommands[normalizedLanguage(target.Language)]
	findings := make([]core.Finding, 0, len(checks))
	for _, check := range checks {
		output, err := env.RunCommandCheck(ctx, target.Path, check)
		if err == nil {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "design.command-check",
			Level:   "fail",
			Message: commandFailureMessage(target, check, output, err),
		}))
	}
	return findings
}

func commandFailureMessage(target core.TargetConfig, check core.CommandCheckConfig, output string, err error) string {
	message := fmt.Sprintf("target %q design command %q failed", target.Name, check.Name)
	output = trimmedOutput(output)
	if output != "" {
		message += ": " + output
	} else if err != nil {
		message += ": " + err.Error()
	}
	return message
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
