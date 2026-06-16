package quality

import (
	"fmt"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func provenancePolicyFindings(env support.Context, findings []core.Finding) []core.Finding {
	cfg := env.Config.Checks.QualityRules.AIProvenance
	if cfg.Enabled != nil && !*cfg.Enabled {
		return nil
	}
	if !aiProvenanceActive(env) {
		return nil
	}
	aiRuleIDs := make([]string, 0)
	for _, finding := range findings {
		if _, ok := aiSlopRuleWeights[finding.RuleID]; !ok {
			continue
		}
		aiRuleIDs = append(aiRuleIDs, finding.RuleID)
	}
	if len(aiRuleIDs) == 0 {
		return nil
	}
	score := scoreFindings(aiRuleIDs)
	warnThreshold := cfg.SlopScoreWarnThreshold
	if warnThreshold == 0 {
		warnThreshold = 20
	}
	if score < warnThreshold {
		return nil
	}
	level := provenancePolicyLevel(cfg, score)
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "quality.ai.provenance-policy",
		Level:   level,
		Path:    "",
		Line:    0,
		Column:  0,
		Message: fmt.Sprintf("AI-assisted provenance is active and the change produced an AI slop score of %d, so stricter review policy applies", score),
	})}
}

func aiProvenanceActive(env support.Context) bool {
	cfg := env.Config.Checks.QualityRules.AIProvenance
	if envFlagEnabled(cfg.EnvVars) {
		return true
	}
	for _, target := range env.Config.Targets {
		if hasCommitTrailer(readGitHeadMessage(target.Path), cfg.CommitTrailers) {
			return true
		}
	}
	return false
}

func provenancePolicyLevel(cfg core.AIProvenanceConfig, score int) string {
	failThreshold := cfg.SlopScoreFailThreshold
	if failThreshold == 0 {
		failThreshold = 40
	}
	if score >= failThreshold {
		return "fail"
	}
	return "warn"
}
