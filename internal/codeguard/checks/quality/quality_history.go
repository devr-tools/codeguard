package quality

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/history"
)

const (
	maintainabilityHighChurnHotspotID  = "maintainability.high-churn-hotspot"
	maintainabilityRepeatDefectAreaID  = "maintainability.repeat-defect-area"
	maintainabilityUnstableInterfaceID = "maintainability.unstable-interface"
	smellShotgunSurgeryHistoryID       = "smell.shotgun-surgery-history"
	smellDivergentChangeHistoryID      = "smell.divergent-change-history"
	maintainabilityChangeAmplifyID     = "maintainability.change-amplification"
	maintainabilityHotspotID           = "maintainability.hotspot"

	historyMaxCommits             = 200
	historyHotspotMinCommits      = 4
	historyHotspotMinChurn        = 30
	historyHighChurnMinCommits    = 5
	historyHighChurnMinChurn      = 50
	historyHighChurnMinComplexity = 8
	historyRepeatDefectMinCommits = 2
	historyCoChangeMinPartners    = 3
	historyCoChangeMinCount       = 2
	historyAmplifierMinPartners   = 4
	historyAmplifierMinEvents     = 6
	historyDivergentMinFamilies   = 3
	historyDivergentMinCommits    = 5
)

var historyDecisionPattern = regexp.MustCompile(`\b(if|else if|for|range|switch|case|catch|except|while|&&|\|\|)\b|\?`)

type fileMaintainabilityHints struct {
	lines         int
	decisionHits  int
	publicSymbols int
}

type coChangePartner struct {
	path  string
	count int
}

func maintainabilityHistoryFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	if env.Mode != core.ScanModeDiff {
		return nil
	}
	changed := changedFilesForTarget(env, target)
	if len(changed) == 0 {
		return nil
	}

	historyCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	report, err := history.CollectChangeMetrics(historyCtx, history.ChangeMetricsOptions{
		RepoPath:   target.Path,
		MaxCommits: historyMaxCommits,
	})
	if err != nil || !report.Available || len(report.Files) == 0 {
		return nil
	}

	findings := make([]core.Finding, 0)
	for _, rel := range changed {
		if isQualityFixturePath(rel) || !qualityPrecisionSupportsFile(target.Language, rel) {
			continue
		}
		metric, ok := report.Files[filepath.ToSlash(rel)]
		if !ok || metric.Commits == 0 {
			continue
		}
		hints := collectMaintainabilityHints(env, target, rel)
		line := deltaFindingLine(env, rel)
		findings = append(findings, historyRuleFindings(env, rel, line, metric, hints)...)
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].RuleID != findings[j].RuleID {
			return findings[i].RuleID < findings[j].RuleID
		}
		return findings[i].Message < findings[j].Message
	})
	return findings
}

func historyRuleFindings(env support.Context, rel string, line int, metric history.FileChangeMetrics, hints fileMaintainabilityHints) []core.Finding {
	findings := make([]core.Finding, 0, 7)
	metadata := historyMetadata(metric, hints)
	topPartners := topCoChangePartners(metric, 1, 5)
	strongPartners := topCoChangePartners(metric, historyCoChangeMinCount, 5)
	subjectFamilies := subjectConcernFamilies(metric.Subjects)

	if metric.Commits >= historyHotspotMinCommits && (metric.Churn >= historyHotspotMinChurn || metric.DefectCommits >= historyRepeatDefectMinCommits) {
		findings = append(findings, historyFinding(env, maintainabilityHotspotID, rel, line,
			fmt.Sprintf("file is a history hotspot: %d commits, %d churn lines, %d defect-linked commits in recent history", metric.Commits, metric.Churn, metric.DefectCommits),
			core.ConfidenceMedium, metadata))
	}
	if metric.Commits >= historyHighChurnMinCommits && metric.Churn >= historyHighChurnMinChurn && complexityScore(hints) >= historyHighChurnMinComplexity {
		findings = append(findings, historyFinding(env, maintainabilityHighChurnHotspotID, rel, line,
			fmt.Sprintf("high-churn hotspot combines %d commits and %d churn lines with complexity hints (%d decision points, %d lines)", metric.Commits, metric.Churn, hints.decisionHits, hints.lines),
			core.ConfidenceHigh, metadata))
	}
	if metric.DefectCommits >= historyRepeatDefectMinCommits {
		findings = append(findings, historyFinding(env, maintainabilityRepeatDefectAreaID, rel, line,
			fmt.Sprintf("file has %d defect-linked commits in recent history; add regression coverage around this change", metric.DefectCommits),
			core.ConfidenceHigh, metadata))
	}
	if hints.publicSymbols > 0 && metric.Commits >= historyHotspotMinCommits && (metric.Churn >= historyHotspotMinChurn || metric.DefectCommits > 0) {
		findings = append(findings, historyFinding(env, maintainabilityUnstableInterfaceID, rel, line,
			fmt.Sprintf("public interface file changed repeatedly (%d commits, %d churn lines, %d public symbols); keep compatibility and callers explicit", metric.Commits, metric.Churn, hints.publicSymbols),
			core.ConfidenceMedium, metadata))
	}
	if len(strongPartners) >= historyCoChangeMinPartners {
		findings = append(findings, historyFinding(env, smellShotgunSurgeryHistoryID, rel, line,
			fmt.Sprintf("file historically changes with %d recurring partners (%s), suggesting shotgun surgery risk", len(strongPartners), formatPartners(strongPartners, 3)),
			core.ConfidenceMedium, metadata))
	}
	if metric.Commits >= historyDivergentMinCommits && len(subjectFamilies) >= historyDivergentMinFamilies {
		findings = append(findings, historyFinding(env, smellDivergentChangeHistoryID, rel, line,
			fmt.Sprintf("file changed for %d concern families in recent history (%s), suggesting divergent-change pressure", len(subjectFamilies), strings.Join(subjectFamilies, ", ")),
			core.ConfidenceMedium, metadata))
	}
	if len(topPartners) >= historyAmplifierMinPartners && totalPartnerEvents(topPartners) >= historyAmplifierMinEvents {
		findings = append(findings, historyFinding(env, maintainabilityChangeAmplifyID, rel, line,
			fmt.Sprintf("changes to this file historically amplify into %d co-change partner events across %d files (%s)", totalPartnerEvents(topPartners), len(topPartners), formatPartners(topPartners, 4)),
			core.ConfidenceMedium, metadata))
	}
	return findings
}

func historyFinding(env support.Context, ruleID string, rel string, line int, message string, confidence string, metadata map[string]string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:     ruleID,
		Level:      "warn",
		Path:       rel,
		Line:       line,
		Column:     1,
		Message:    message,
		Confidence: confidence,
		Metadata:   metadata,
	})
}

func collectMaintainabilityHints(env support.Context, target core.TargetConfig, rel string) fileMaintainabilityHints {
	data, ok := readCurrentTargetFile(env, target, rel)
	if !ok {
		return fileMaintainabilityHints{}
	}
	source := string(data)
	return fileMaintainabilityHints{
		lines:         env.CountLines(data),
		decisionHits:  len(historyDecisionPattern.FindAllString(source, -1)),
		publicSymbols: publicSurfaceCount(target.Language, rel, source),
	}
}

func complexityScore(hints fileMaintainabilityHints) int {
	score := hints.decisionHits
	switch {
	case hints.lines >= 120:
		score += 4
	case hints.lines >= 80:
		score += 3
	case hints.lines >= 50:
		score += 2
	}
	return score
}

func historyMetadata(metric history.FileChangeMetrics, hints fileMaintainabilityHints) map[string]string {
	partners := topCoChangePartners(metric, 1, 5)
	return map[string]string{
		"commits":            strconv.Itoa(metric.Commits),
		"churn":              strconv.Itoa(metric.Churn),
		"additions":          strconv.Itoa(metric.Additions),
		"deletions":          strconv.Itoa(metric.Deletions),
		"defect_commits":     strconv.Itoa(metric.DefectCommits),
		"co_change_partners": strconv.Itoa(len(metric.CoChangePartners)),
		"top_partners":       formatPartners(partners, 5),
		"lines":              strconv.Itoa(hints.lines),
		"decision_hints":     strconv.Itoa(hints.decisionHits),
		"public_symbols":     strconv.Itoa(hints.publicSymbols),
	}
}

func topCoChangePartners(metric history.FileChangeMetrics, minCount int, limit int) []coChangePartner {
	partners := make([]coChangePartner, 0, len(metric.CoChangePartners))
	for path, count := range metric.CoChangePartners {
		if count >= minCount {
			partners = append(partners, coChangePartner{path: path, count: count})
		}
	}
	sort.Slice(partners, func(i, j int) bool {
		if partners[i].count != partners[j].count {
			return partners[i].count > partners[j].count
		}
		return partners[i].path < partners[j].path
	})
	if limit > 0 && len(partners) > limit {
		return partners[:limit]
	}
	return partners
}

func formatPartners(partners []coChangePartner, limit int) string {
	if limit > 0 && len(partners) > limit {
		partners = partners[:limit]
	}
	parts := make([]string, 0, len(partners))
	for _, partner := range partners {
		parts = append(parts, fmt.Sprintf("%s:%d", partner.path, partner.count))
	}
	return strings.Join(parts, ", ")
}

func totalPartnerEvents(partners []coChangePartner) int {
	total := 0
	for _, partner := range partners {
		total += partner.count
	}
	return total
}

func subjectConcernFamilies(subjects []string) []string {
	families := map[string]struct{}{}
	for _, subject := range subjects {
		for _, family := range subjectFamilies(subject) {
			families[family] = struct{}{}
		}
	}
	out := make([]string, 0, len(families))
	for family := range families {
		out = append(out, family)
	}
	sort.Strings(out)
	return out
}

func subjectFamilies(subject string) []string {
	lower := strings.ToLower(subject)
	families := make([]string, 0, 2)
	for _, candidate := range []struct {
		name  string
		terms []string
	}{
		{name: "api", terms: []string{"api", "interface", "contract", "endpoint", "schema"}},
		{name: "build", terms: []string{"build", "ci", "pipeline", "dependency", "deps"}},
		{name: "data", terms: []string{"db", "database", "migration", "query", "cache"}},
		{name: "docs", terms: []string{"doc", "readme", "comment"}},
		{name: "defect", terms: []string{"fix", "bug", "hotfix", "regression", "incident", "broken"}},
		{name: "performance", terms: []string{"perf", "latency", "speed", "memory"}},
		{name: "refactor", terms: []string{"refactor", "cleanup", "rename", "move"}},
		{name: "test", terms: []string{"test", "spec", "coverage"}},
		{name: "ui", terms: []string{"ui", "view", "component", "style", "css"}},
	} {
		for _, term := range candidate.terms {
			if strings.Contains(lower, term) {
				families = append(families, candidate.name)
				break
			}
		}
	}
	if len(families) == 0 {
		families = append(families, "behavior")
	}
	return families
}
