package quality

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func changeRiskFindings(env support.Context, target core.TargetConfig, findings []core.Finding) []core.Finding {
	cfg := env.Config.Checks.QualityRules.AIChangeRisk
	if cfg.Enabled != nil && !*cfg.Enabled {
		return nil
	}
	artifact, ok := aiChangeRiskArtifact(env, target, findings)
	if !ok {
		return nil
	}
	if env.PutArtifact != nil {
		env.PutArtifact(artifact)
	}
	warnThreshold := cfg.WarnThreshold
	if warnThreshold == 0 {
		warnThreshold = 30
	}
	if artifact.ChangeRisk.Score < warnThreshold {
		return nil
	}
	level := artifact.ChangeRisk.Level
	if level == "" {
		level = "warn"
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "quality.ai.change-risk",
		Level:   level,
		Path:    "",
		Message: fmt.Sprintf("AI change risk score %d/%d for target %q: %s", artifact.ChangeRisk.Score, 100, target.Name, summarizeChangeRisk(artifact.ChangeRisk)),
	})}
}

func aiChangeRiskArtifact(env support.Context, target core.TargetConfig, findings []core.Finding) (core.Artifact, bool) {
	aiRuleIDs, semanticCount, coverageGap := summarizeChangeRiskInputs(findings)
	provenanceActive := aiProvenanceActive(env)
	if len(aiRuleIDs) == 0 && !provenanceActive && !coverageGap {
		return core.Artifact{}, false
	}
	score := 0
	components := make([]core.ChangeRiskComponent, 0, 5)
	changedFiles := targetChangedFileCount(env, target)
	score, components = addAISignalRisk(score, components, aiRuleIDs)
	score, components = addProvenanceRisk(score, components, provenanceActive)
	score, components = addDiffBreadthRisk(score, components, changedFiles)
	score, components = addSemanticRisk(score, components, semanticCount)
	score, components = addCoverageGapRisk(score, components, coverageGap)
	score = minInt(score, 100)
	if score == 0 {
		return core.Artifact{}, false
	}
	language := support.NormalizedLanguage(target.Language)
	if language == "" {
		language = "go"
	}
	risk := core.ChangeRiskArtifact{
		Score:                score,
		Level:                changeRiskLevel(env.Config.Checks.QualityRules.AIChangeRisk, score),
		ProvenanceActive:     provenanceActive,
		ChangedFiles:         changedFiles,
		HighImpactChange:     changedFiles >= 5,
		AIFindingCount:       len(aiRuleIDs),
		SemanticFindingCount: semanticCount,
		Components:           components,
	}
	return support.NewChangeRiskArtifact("change_risk."+language+"."+artifactSafeID(target.Name), language, target.Path, risk), true
}

func changeRiskLevel(cfg core.AIChangeRiskConfig, score int) string {
	failThreshold := cfg.FailThreshold
	if failThreshold == 0 {
		failThreshold = 60
	}
	if score >= failThreshold {
		return "fail"
	}
	return "warn"
}

func summarizeChangeRisk(risk *core.ChangeRiskArtifact) string {
	if risk == nil || len(risk.Components) == 0 {
		return "risk signals accumulated beyond the configured threshold"
	}
	parts := make([]string, 0, len(risk.Components))
	for _, component := range risk.Components {
		if component.Detail != "" {
			parts = append(parts, component.Detail)
			continue
		}
		parts = append(parts, component.Label)
	}
	return strings.Join(parts, "; ")
}

func targetChangedFileCount(env support.Context, _ core.TargetConfig) int {
	return len(env.ChangedFiles)
}

func summarizeChangeRiskInputs(findings []core.Finding) ([]string, int, bool) {
	aiRuleIDs := make([]string, 0)
	semanticCount := 0
	coverageGap := false
	for _, finding := range findings {
		if _, ok := aiSlopRuleWeights[finding.RuleID]; ok {
			aiRuleIDs = append(aiRuleIDs, finding.RuleID)
		}
		if strings.HasPrefix(finding.RuleID, "quality.ai.semantic-") && finding.RuleID != "quality.ai.semantic-runtime" {
			semanticCount++
		}
		if finding.RuleID == "quality.coverage-delta" {
			coverageGap = true
		}
	}
	return aiRuleIDs, semanticCount, coverageGap
}

func addAISignalRisk(score int, components []core.ChangeRiskComponent, aiRuleIDs []string) (int, []core.ChangeRiskComponent) {
	if len(aiRuleIDs) == 0 {
		return score, components
	}
	slop := scoreFindings(aiRuleIDs)
	contribution := minInt(slop/2, 50)
	score += contribution
	components = append(components, core.ChangeRiskComponent{
		Label:        "ai_signals",
		Contribution: contribution,
		Detail:       fmt.Sprintf("%d AI findings contributed a slop score of %d", len(aiRuleIDs), slop),
	})
	return score, components
}

func addProvenanceRisk(score int, components []core.ChangeRiskComponent, provenanceActive bool) (int, []core.ChangeRiskComponent) {
	if !provenanceActive {
		return score, components
	}
	score += 15
	components = append(components, core.ChangeRiskComponent{
		Label:        "provenance",
		Contribution: 15,
		Detail:       "AI-assisted provenance is active for the current change",
	})
	return score, components
}

func addDiffBreadthRisk(score int, components []core.ChangeRiskComponent, changedFiles int) (int, []core.ChangeRiskComponent) {
	contribution := 0
	switch {
	case changedFiles >= 5:
		contribution = 10
	case changedFiles >= 2:
		contribution = 5
	}
	if contribution == 0 {
		return score, components
	}
	score += contribution
	components = append(components, core.ChangeRiskComponent{
		Label:        "diff_breadth",
		Contribution: contribution,
		Detail:       fmt.Sprintf("%d changed files fall under this target", changedFiles),
	})
	return score, components
}

func addSemanticRisk(score int, components []core.ChangeRiskComponent, semanticCount int) (int, []core.ChangeRiskComponent) {
	if semanticCount == 0 {
		return score, components
	}
	contribution := minInt(semanticCount*5, 15)
	score += contribution
	components = append(components, core.ChangeRiskComponent{
		Label:        "semantic_findings",
		Contribution: contribution,
		Detail:       fmt.Sprintf("%d semantic AI findings were reported", semanticCount),
	})
	return score, components
}

func addCoverageGapRisk(score int, components []core.ChangeRiskComponent, coverageGap bool) (int, []core.ChangeRiskComponent) {
	if !coverageGap {
		return score, components
	}
	score += 10
	components = append(components, core.ChangeRiskComponent{
		Label:        "coverage_gap",
		Contribution: 10,
		Detail:       "changed-line coverage enforcement reported a gap",
	})
	return score, components
}
