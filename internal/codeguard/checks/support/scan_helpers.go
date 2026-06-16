package support

import (
	"context"
	"regexp"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func CollectTargetFindings(ctx context.Context, env Context, collect func(context.Context, Context, core.TargetConfig) []core.Finding) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
		findings = append(findings, collect(ctx, env, target)...)
	}
	return findings
}

// RunTargetSection collects findings for every configured target through
// perTarget and finalizes the section with the given id and title.
func RunTargetSection(ctx context.Context, env Context, id string, title string, perTarget func(context.Context, Context, core.TargetConfig) []core.Finding) core.SectionResult {
	return env.FinalizeSection(id, title, CollectTargetFindings(ctx, env, perTarget))
}

type SectionCommandSpec struct {
	Checks  []core.CommandCheckConfig
	RuleID  string
	Section string
}

func SectionCommandFindings(ctx context.Context, env Context, target core.TargetConfig, spec SectionCommandSpec) []core.Finding {
	return RunCommandChecks(ctx, env, target, spec.Checks, func(check core.CommandCheckConfig, output string, err error) core.Finding {
		return env.NewFinding(FindingInput{
			RuleID:  spec.RuleID,
			Level:   "fail",
			Message: CommandFailureMessage(spec.Section, target, check, output, err),
		})
	})
}

func SectionDiffCommandFindings(ctx context.Context, env Context, target core.TargetConfig, spec SectionCommandSpec) []core.Finding {
	return RunDiffCommandChecks(ctx, env, target, spec.Checks, func(check core.CommandCheckConfig, output string, err error) core.Finding {
		return env.NewFinding(FindingInput{
			RuleID:  spec.RuleID,
			Level:   "fail",
			Message: DiffCommandFailureMessage(spec.Section, target, check, output, err),
		})
	})
}

func RegexLineFindings(ctx ScriptScanContext, pattern *regexp.Regexp, build func(int) core.Finding) []core.Finding {
	matches := pattern.FindAllStringIndex(ctx.Code, -1)
	if len(matches) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0, len(matches))
	seenLines := make(map[int]struct{}, len(matches))
	for _, match := range matches {
		line := LineNumberForOffset(ctx.Source, match[0])
		if _, exists := seenLines[line]; exists {
			continue
		}
		seenLines[line] = struct{}{}
		findings = append(findings, build(line))
	}
	return findings
}

type ScriptScanContext struct {
	Source string
	Code   string
}

type ScriptRegexSpec struct {
	Pattern *regexp.Regexp
	RuleID  string
	Level   string
	Message string
}

func ScriptRegexFindings(env Context, file string, ctx ScriptScanContext, spec ScriptRegexSpec) []core.Finding {
	return RegexLineFindings(ctx, spec.Pattern, func(line int) core.Finding {
		return env.NewFinding(FindingInput{
			RuleID:  spec.RuleID,
			Level:   spec.Level,
			Path:    file,
			Line:    line,
			Column:  1,
			Message: spec.Message,
		})
	})
}
