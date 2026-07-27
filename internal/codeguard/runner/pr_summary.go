package runner

import (
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func addPRSummaryArtifact(sc runnersupport.Context, sections []core.SectionResult) {
	if sc.Opts.Mode != core.ScanModeDiff {
		return
	}
	summary := core.PRSummaryArtifact{}
	cfg := sc.Cfg.Checks.ProductionRisk
	if cfg.Enabled != nil && *cfg.Enabled {
		summary.ProductionRisk = productionRiskMetric(cfg, sections)
	}
	summary.ChangeSafety = findingFamilyMetric(sections, changeSafetyMetricRules())
	summary.MaintainabilityDelta = findingFamilyMetric(sections, maintainabilityMetricRules())
	summary.RefactorConfidence = findingFamilyMetric(sections, refactorConfidenceMetricRules())
	if summary.ProductionRisk == nil && summary.ChangeSafety == nil && summary.MaintainabilityDelta == nil && summary.RefactorConfidence == nil {
		return
	}
	sc.Artifacts.Put(core.Artifact{
		ID:        "pr_summary",
		Kind:      core.ReportArtifactKindPRSummary,
		PRSummary: &summary,
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

type prSummaryMetricRule struct {
	Label  string
	Detail string
	Match  func(core.Finding) bool
}

func findingFamilyMetric(sections []core.SectionResult, rules []prSummaryMetricRule) *core.PRSummaryMetric {
	componentsByLabel := map[string]*core.PRSummaryComponent{}
	for _, section := range sections {
		for _, finding := range section.Findings {
			rule, ok := matchPRSummaryMetricRule(finding, rules)
			if !ok {
				continue
			}
			weight := prSummaryFindingWeight(finding)
			if weight == 0 {
				continue
			}
			component := componentsByLabel[rule.Label]
			if component == nil {
				component = &core.PRSummaryComponent{
					Label:  rule.Label,
					Weight: weight,
					Detail: rule.Detail,
				}
				componentsByLabel[rule.Label] = component
			}
			component.Count++
			component.Contribution += weight
			if weight > component.Weight {
				component.Weight = weight
			}
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
	sortPRSummaryComponents(components)
	score := minRiskScore(total)
	return &core.PRSummaryMetric{
		Score:      score,
		Level:      prSummaryRiskLevel(score),
		Components: components,
	}
}

func matchPRSummaryMetricRule(finding core.Finding, rules []prSummaryMetricRule) (prSummaryMetricRule, bool) {
	for _, rule := range rules {
		if rule.Match(finding) {
			return rule, true
		}
	}
	return prSummaryMetricRule{}, false
}

func prSummaryFindingWeight(finding core.Finding) int {
	weight := 0
	switch strings.ToLower(finding.Level) {
	case "fail", "error":
		weight = 35
	case "warn", "warning":
		weight = 18
	default:
		weight = 10
	}
	switch strings.ToLower(finding.Confidence) {
	case "high":
		weight += 5
	case "low":
		weight -= 5
	}
	if weight < 0 {
		return 0
	}
	return weight
}

func prSummaryRiskLevel(score int) string {
	switch {
	case score >= 70:
		return "fail"
	case score >= 35:
		return "warn"
	default:
		return "pass"
	}
}

func sortPRSummaryComponents(components []core.PRSummaryComponent) {
	sort.Slice(components, func(i, j int) bool {
		if components[i].Contribution != components[j].Contribution {
			return components[i].Contribution > components[j].Contribution
		}
		if components[i].Label != components[j].Label {
			return components[i].Label < components[j].Label
		}
		return components[i].Detail < components[j].Detail
	})
}

func changeSafetyMetricRules() []prSummaryMetricRule {
	return []prSummaryMetricRule{
		{Label: "change_scope", Detail: "change concentration, size, or concern-mixing findings", Match: ruleIDHasPrefix("change.")},
		{Label: "test_evidence", Detail: "testability findings in changed behavior", Match: ruleIDHasPrefix("testing.")},
	}
}

func maintainabilityMetricRules() []prSummaryMetricRule {
	return []prSummaryMetricRule{
		{Label: "maintainability", Detail: "maintainability delta findings in changed code", Match: ruleIDHasPrefix("maintainability.")},
		{Label: "code_quality", Detail: "local quality findings that affect maintainability", Match: ruleIDHasAnyPrefix("quality.", "smell.", "naming.", "function.")},
		{Label: "defensive_programming", Detail: "error-handling or defensive-programming findings in changed code", Match: ruleIDHasAnyPrefix("error.", "defensive.")},
	}
}

func refactorConfidenceMetricRules() []prSummaryMetricRule {
	return []prSummaryMetricRule{
		{Label: "behavior_preservation", Detail: "refactor findings that indicate observable behavior may have changed", Match: ruleIDHasPrefix("refactor.")},
		{Label: "mixed_refactor", Detail: "change findings that weaken refactor confidence", Match: ruleIDEquals("change.mixed-refactor-and-behavior", "change.move-without-verification")},
	}
}

func ruleIDHasPrefix(prefix string) func(core.Finding) bool {
	return func(finding core.Finding) bool {
		return strings.HasPrefix(finding.RuleID, prefix)
	}
}

func ruleIDHasAnyPrefix(prefixes ...string) func(core.Finding) bool {
	return func(finding core.Finding) bool {
		for _, prefix := range prefixes {
			if strings.HasPrefix(finding.RuleID, prefix) {
				return true
			}
		}
		return false
	}
}

func ruleIDEquals(ids ...string) func(core.Finding) bool {
	return func(finding core.Finding) bool {
		for _, id := range ids {
			if finding.RuleID == id {
				return true
			}
		}
		return false
	}
}
