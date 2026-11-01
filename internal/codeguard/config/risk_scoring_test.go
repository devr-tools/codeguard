package config

import (
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestApplyDefaultsSetsStableRiskScoringWeights(t *testing.T) {
	cfg := core.Config{}
	ApplyDefaults(&cfg)
	risk := cfg.Checks.QualityRules.RiskScoring
	if risk.Enabled == nil || !*risk.Enabled {
		t.Fatalf("risk scoring enabled = %v, want true", risk.Enabled)
	}
	if risk.MaxHotspots != 5 || risk.ChangedFileWeight != 5 || risk.FailFindingWeight != 30 || risk.WarnFindingWeight != 15 || risk.SlopScoreDivisor != 10 {
		t.Fatalf("unexpected risk scoring defaults: %#v", risk)
	}
}

func TestValidateRiskScoringRejectsNegativeWeights(t *testing.T) {
	err := validateRiskScoring(core.RiskScoringConfig{SecurityWeight: -1})
	if err == nil || !strings.Contains(err.Error(), "security_weight") {
		t.Fatalf("error = %v, want security_weight validation error", err)
	}
}
