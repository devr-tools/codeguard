package quality

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const coverageDeltaRuleID = "quality.coverage-delta"

// coverageProfile maps a target-relative slash path to per-line hit counts.
// Lines absent from the map were not measurable (comments, declarations, or
// files outside the coverage report).
type coverageProfile map[string]map[int]int

func coverageDeltaFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	cfg := env.Config.Checks.QualityRules.CoverageDelta
	if cfg.Enabled == nil || !*cfg.Enabled || env.ScanMode != core.ScanModeDiff || env.DiffScope == nil {
		return nil
	}
	scope := env.DiffScope()
	if len(scope) == 0 {
		return nil
	}
	profile, skip, err := coverageProfileForTarget(ctx, env, target, cfg, scope)
	if skip {
		return nil
	}
	if err != nil {
		return []core.Finding{env.NewFinding(support.FindingInput{
			RuleID:  coverageDeltaRuleID,
			Level:   "warn",
			Message: fmt.Sprintf("target %q coverage run failed: %s", target.Name, trimmedOutput(err.Error())),
		})}
	}
	return changedLineCoverageFindings(env, cfg, scope, profile)
}

func coverageProfileForTarget(ctx context.Context, env support.Context, target core.TargetConfig, cfg core.CoverageDeltaConfig, scope map[string]core.ChangedLineRanges) (coverageProfile, bool, error) {
	language := normalizedLanguage(target.Language)
	if language == "" || language == "go" {
		profile, err := goCoverageProfile(ctx, env, target, scope)
		return profile, profile == nil && err == nil, err
	}
	command, ok := cfg.LanguageCommands[language]
	if !ok {
		return nil, true, nil
	}
	profile, err := commandCoverageProfile(ctx, env, target, command)
	return profile, false, err
}

func changedLineCoverageFindings(env support.Context, cfg core.CoverageDeltaConfig, scope map[string]core.ChangedLineRanges, profile coverageProfile) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, rel := range sortedScopePaths(scope) {
		hits, ok := profile[rel]
		if !ok {
			continue
		}
		covered, uncovered := changedLineCoverage(scope[rel], hits)
		total := covered + len(uncovered)
		if total == 0 {
			continue
		}
		pct := covered * 100 / total
		if pct >= *cfg.MinChangedLineCoverage {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID: coverageDeltaRuleID,
			Level:  coverageLevel(cfg, pct),
			Path:   rel,
			Line:   uncovered[0],
			Message: fmt.Sprintf("changed-line coverage %d%% is below threshold %d%% (%d of %d measurable changed lines uncovered): lines %s",
				pct, *cfg.MinChangedLineCoverage, len(uncovered), total, formatLineList(uncovered)),
		}))
	}
	return findings
}

func coverageLevel(cfg core.CoverageDeltaConfig, pct int) string {
	if cfg.FailUnder != nil && pct < *cfg.FailUnder {
		return "fail"
	}
	return "warn"
}

func changedLineCoverage(ranges core.ChangedLineRanges, hits map[int]int) (int, []int) {
	covered := 0
	uncovered := make([]int, 0)
	for line, count := range hits {
		if !ranges.Contains(line) {
			continue
		}
		if count > 0 {
			covered++
		} else {
			uncovered = append(uncovered, line)
		}
	}
	sort.Ints(uncovered)
	return covered, uncovered
}

func sortedScopePaths(scope map[string]core.ChangedLineRanges) []string {
	paths := make([]string, 0, len(scope))
	for path := range scope {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func formatLineList(lines []int) string {
	const maxSegments = 10
	segments := make([]string, 0)
	for idx := 0; idx < len(lines); {
		end := idx
		for end+1 < len(lines) && lines[end+1] == lines[end]+1 {
			end++
		}
		if end > idx {
			segments = append(segments, fmt.Sprintf("%d-%d", lines[idx], lines[end]))
		} else {
			segments = append(segments, fmt.Sprintf("%d", lines[idx]))
		}
		idx = end + 1
		if len(segments) == maxSegments && idx < len(lines) {
			segments = append(segments, "...")
			break
		}
	}
	return strings.Join(segments, ", ")
}
