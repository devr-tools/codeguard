package runner

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// addRiskArtifacts produces review-priority data only for diff scans. It is a
// report postprocessor so it can reuse the final, baseline-filtered findings
// and artifacts without changing check severity or rerunning any analysis.
func addRiskArtifacts(sc runnersupport.Context, sections []core.SectionResult) {
	if sc.Opts.Mode != core.ScanModeDiff || sc.Cfg.Checks.QualityRules.RiskScoring.Enabled == nil || !*sc.Cfg.Checks.QualityRules.RiskScoring.Enabled {
		return
	}
	paths := runnersupport.ChangedDiffFiles(sc)
	if len(paths) == 0 {
		return
	}
	cfg := sc.Cfg.Checks.QualityRules.RiskScoring
	entries := make(map[string]*core.FileRiskEntry, len(paths))
	for _, path := range paths {
		entry := &core.FileRiskEntry{Path: path}
		addRiskComponent(entry, "changed_file", cfg.ChangedFileWeight, "file is in the diff")
		entries[path] = entry
	}

	for _, section := range sections {
		for _, finding := range section.Findings {
			entry := entries[cleanRiskPath(finding.Path)]
			if entry == nil || finding.Suppressed {
				continue
			}
			addFindingRisk(entry, finding, cfg)
		}
	}
	addArtifactRisk(entries, sc.Artifacts.List(), cfg)

	ranked := make([]core.FileRiskEntry, 0, len(entries))
	for _, entry := range entries {
		entry.Score = minRiskScore(sumRiskComponents(entry.Components))
		sortRiskComponents(entry.Components)
		ranked = append(ranked, *entry)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].Path < ranked[j].Path
	})
	for i := range ranked {
		ranked[i].Rank = i + 1
	}

	sc.Artifacts.Put(core.Artifact{ID: "file_risk", Kind: core.ReportArtifactKindFileRisk, FileRisk: &core.FileRiskArtifact{Files: ranked}})
	limit := cfg.MaxHotspots
	if limit > len(ranked) {
		limit = len(ranked)
	}
	hotspots := append([]core.FileRiskEntry(nil), ranked[:limit]...)
	sc.Artifacts.Put(core.Artifact{ID: "pr_hotspots", Kind: core.ReportArtifactKindPRHotspots, PRHotspots: &core.PRHotspotsArtifact{Hotspots: hotspots}})
}

func addFindingRisk(entry *core.FileRiskEntry, finding core.Finding, cfg core.RiskScoringConfig) {
	switch strings.ToLower(finding.Level) {
	case "fail", "error":
		addRiskComponent(entry, "fail_finding", cfg.FailFindingWeight, finding.RuleID)
	case "warn", "warning":
		addRiskComponent(entry, "warn_finding", cfg.WarnFindingWeight, finding.RuleID)
	}
	if finding.Section == "security" || strings.HasPrefix(finding.RuleID, "security.") {
		addRiskComponent(entry, "security_finding", cfg.SecurityWeight, finding.RuleID)
	}
	if finding.Section == "supply_chain" || strings.HasPrefix(finding.RuleID, "supply_chain.") {
		addRiskComponent(entry, "supply_chain_finding", cfg.SupplyChainWeight, finding.RuleID)
	}
	if finding.RuleID == "quality.coverage-delta" {
		addRiskComponent(entry, "coverage_gap", cfg.CoverageGapWeight, "changed-line coverage is below policy")
	}
	if strings.HasPrefix(finding.RuleID, "quality.ai.") && finding.RuleID != "quality.ai.provenance-policy" {
		addRiskComponent(entry, "ai_signal", cfg.AISignalWeight, finding.RuleID)
	}
}

func addArtifactRisk(entries map[string]*core.FileRiskEntry, artifacts []core.Artifact, cfg core.RiskScoringConfig) {
	for _, artifact := range artifacts {
		paths := riskArtifactPaths(entries, artifact.Target)
		if len(paths) == 0 {
			continue
		}
		if divisor := core.IntValue(cfg.SlopScoreDivisor); artifact.SlopScore != nil && divisor > 0 {
			weight := artifact.SlopScore.Score / divisor
			if weight > 0 {
				for _, path := range paths {
					entries[path].Components = append(entries[path].Components, riskComponent("slop_score", weight, "target AI-quality score"))
				}
			}
		}
		if artifact.ChangeRisk != nil && artifact.ChangeRisk.ProvenanceActive {
			for _, path := range paths {
				addRiskComponent(entries[path], "ai_provenance", cfg.AIProvenanceWeight, "AI-assisted provenance is active for this target")
			}
		}
	}
}

func riskArtifactPaths(entries map[string]*core.FileRiskEntry, target string) []string {
	target = cleanRiskPath(target)
	paths := make([]string, 0, len(entries))
	for path := range entries {
		if target == "" || target == "." || path == target || strings.HasPrefix(path, target+"/") {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

// addRiskComponent records a weighted signal on entry. A weight explicitly
// configured as 0 means the signal is switched off, so it adds no
// zero-contribution noise to the artifact.
func addRiskComponent(entry *core.FileRiskEntry, label string, weight *int, detail string) {
	if core.IntValue(weight) == 0 {
		return
	}
	entry.Components = append(entry.Components, riskComponent(label, core.IntValue(weight), detail))
}

func riskComponent(label string, weight int, detail string) core.FileRiskComponent {
	return core.FileRiskComponent{Label: label, Weight: weight, Count: 1, Contribution: weight, Detail: detail}
}

func sumRiskComponents(components []core.FileRiskComponent) int {
	total := 0
	for _, component := range components {
		total += component.Contribution
	}
	return total
}

func sortRiskComponents(components []core.FileRiskComponent) {
	sort.Slice(components, func(i, j int) bool {
		if components[i].Label != components[j].Label {
			return components[i].Label < components[j].Label
		}
		return components[i].Detail < components[j].Detail
	})
}

func cleanRiskPath(path string) string {
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if path == "." {
		return path
	}
	return strings.TrimPrefix(path, "./")
}

func minRiskScore(score int) int {
	if score > 100 {
		return 100
	}
	return score
}
