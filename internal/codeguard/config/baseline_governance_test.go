package config

import (
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestValidateBaselineGovernanceRejectsInvalidValues(t *testing.T) {
	cfg := ExampleConfig()
	cfg.Baseline.Governance.MaxEntries = -1
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "max_entries") {
		t.Fatalf("Validate error = %v", err)
	}

	cfg = ExampleConfig()
	cfg.Baseline.Governance.Ownership = []core.BaselineOwnershipConfig{{Pattern: "services/**", Owner: ""}}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("Validate error = %v", err)
	}
}

func TestValidateBaselineGovernanceAllowsOmittedAndValidPolicy(t *testing.T) {
	for _, governance := range []core.BaselineGovernanceConfig{
		{},
		{MaxEntries: 10, ForbidGrowth: true, RequireNoStaleEntries: true, ProhibitedNewRulePrefixes: []string{"security."}, SampleLimit: 3, Ownership: []core.BaselineOwnershipConfig{{Pattern: "services/**", Owner: "services"}}},
	} {
		cfg := ExampleConfig()
		cfg.Baseline.Governance = governance
		if err := Validate(cfg); err != nil {
			t.Fatalf("Validate(%#v): %v", governance, err)
		}
	}
}
