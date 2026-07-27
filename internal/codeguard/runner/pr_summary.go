package runner

import (
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func addPRSummaryArtifact(sc runnersupport.Context, sections []core.SectionResult) {
	cfg := sc.Cfg.Checks.ProductionRisk
	if cfg.Enabled == nil || !*cfg.Enabled || sc.Opts.Mode != core.ScanModeDiff {
		return
	}
	metric := productionRiskMetric(cfg, sections)
	if metric == nil {
		return
	}
	sc.Artifacts.Put(core.Artifact{
		ID:   "pr_summary",
		Kind: core.ReportArtifactKindPRSummary,
		PRSummary: &core.PRSummaryArtifact{
			ProductionRisk: metric,
		},
	})
}

func productionRiskMetric(cfg core.ProductionRiskConfig, sections []core.SectionResult) *core.PRSummaryMetric {
	componentsByLabel := map[string]*core.PRSummaryComponent{}
	for _, section := range sections {
		for _, finding := range section.Findings {
			weight := 0
			switch {
			case strings.HasPrefix(finding.RuleID, "reliability."):
				weight += cfg.ReliabilityWeight
			case strings.HasPrefix(finding.RuleID, "data."):
				weight += cfg.DataWeight
			case finding.RuleID == "contracts.non-expand-contract-migration":
				weight += cfg.DataWeight
			default:
				continue
			}
			switch strings.ToLower(finding.Level) {
			case "fail", "error":
				weight += cfg.FailWeight
			case "warn", "warning":
				weight += cfg.WarnWeight
			}
			label := productionRiskLabel(finding)
			component := componentsByLabel[label]
			if component == nil {
				component = &core.PRSummaryComponent{
					Label:  label,
					Weight: weight,
					Detail: productionRiskDetail(finding),
				}
				componentsByLabel[label] = component
			}
			component.Count++
			component.Contribution += weight
		}
	}
	if len(componentsByLabel) == 0 {
		return nil
	}
	components := make([]core.PRSummaryComponent, 0, len(componentsByLabel))
	total := 0
	for _, component := range componentsByLabel {
		components = append(components, *component)
		total += component.Contribution
	}
	sort.Slice(components, func(i, j int) bool {
		if components[i].Contribution != components[j].Contribution {
			return components[i].Contribution > components[j].Contribution
		}
		return components[i].Label < components[j].Label
	})
	score := minRiskScore(total)
	return &core.PRSummaryMetric{
		Score:      score,
		Level:      productionRiskLevel(score, cfg),
		Components: components,
	}
}

func productionRiskLabel(finding core.Finding) string {
	if strings.HasPrefix(finding.RuleID, "reliability.") {
		return "reliability"
	}
	if strings.HasPrefix(finding.RuleID, "data.") || finding.RuleID == "contracts.non-expand-contract-migration" {
		return "data_correctness"
	}
	return "production"
}

func productionRiskDetail(finding core.Finding) string {
	if strings.HasPrefix(finding.RuleID, "reliability.") {
		return "active reliability findings in changed code"
	}
	if strings.HasPrefix(finding.RuleID, "data.") {
		return "active data-correctness findings in changed code"
	}
	if finding.RuleID == "contracts.non-expand-contract-migration" {
		return "unsafe rolling schema migration finding"
	}
	return finding.RuleID
}

func productionRiskLevel(score int, cfg core.ProductionRiskConfig) string {
	switch {
	case cfg.FailThreshold > 0 && score >= cfg.FailThreshold:
		return "fail"
	case cfg.WarnThreshold > 0 && score >= cfg.WarnThreshold:
		return "warn"
	default:
		return "pass"
	}
}
