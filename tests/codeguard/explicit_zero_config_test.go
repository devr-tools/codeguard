package codeguard_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// Risk weights are plain ints defaulted with "0 means unset", which used to
// mean a user could not switch one signal off: `security_weight: 0` silently
// became 10. The config types now record which weight keys a file states, so
// an explicit 0 survives defaulting; these tests pin both directions in YAML
// and JSON.

const explicitZeroYAML = `name: zero
targets:
  - name: app
    path: .
    language: go
checks:
  quality_rules:
    risk_scoring:
      security_weight: 0
      slop_score_divisor: 0
      fail_finding_weight: 7
  production_risk:
    fail_weight: 0
    data_weight: 3
`

const explicitZeroJSON = `{
  "name": "zero",
  "targets": [{"name": "app", "path": ".", "language": "go"}],
  "checks": {
    "quality_rules": {"risk_scoring": {"security_weight": 0, "slop_score_divisor": 0, "fail_finding_weight": 7}},
    "production_risk": {"fail_weight": 0, "data_weight": 3}
  }
}`

func TestExplicitZeroRiskWeightsSurviveDefaulting(t *testing.T) {
	for _, tc := range []struct {
		name, file, body string
	}{
		{"yaml", "codeguard.yaml", explicitZeroYAML},
		{"json", "codeguard.json", explicitZeroJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := loadConfigFixture(t, tc.file, tc.body)
			risk := cfg.Checks.QualityRules.RiskScoring
			assertIntSetting(t, "security_weight", risk.SecurityWeight, 0)
			assertIntSetting(t, "slop_score_divisor", risk.SlopScoreDivisor, 0)
			assertIntSetting(t, "fail_finding_weight", risk.FailFindingWeight, 7)
			// Unset siblings still take their defaults.
			assertIntSetting(t, "warn_finding_weight", risk.WarnFindingWeight, 15)
			assertIntSetting(t, "supply_chain_weight", risk.SupplyChainWeight, 10)
			assertIntSetting(t, "changed_file_weight", risk.ChangedFileWeight, 5)

			prod := cfg.Checks.ProductionRisk
			assertIntSetting(t, "production_risk.fail_weight", prod.FailWeight, 0)
			assertIntSetting(t, "production_risk.data_weight", prod.DataWeight, 3)
			assertIntSetting(t, "production_risk.warn_weight", prod.WarnWeight, 10)
			assertIntSetting(t, "production_risk.reliability_weight", prod.ReliabilityWeight, 12)
		})
	}
}

func TestUnsetRiskWeightsTakeDocumentedDefaults(t *testing.T) {
	cfg := loadConfigFixture(t, "codeguard.yaml", "name: defaults\ntargets:\n  - name: app\n    path: .\n    language: go\n")
	risk := cfg.Checks.QualityRules.RiskScoring
	for name, want := range map[string]struct {
		got  int
		want int
	}{
		"changed_file_weight":  {risk.ChangedFileWeight, 5},
		"fail_finding_weight":  {risk.FailFindingWeight, 30},
		"warn_finding_weight":  {risk.WarnFindingWeight, 15},
		"security_weight":      {risk.SecurityWeight, 10},
		"supply_chain_weight":  {risk.SupplyChainWeight, 10},
		"coverage_gap_weight":  {risk.CoverageGapWeight, 15},
		"ai_provenance_weight": {risk.AIProvenanceWeight, 15},
		"ai_signal_weight":     {risk.AISignalWeight, 10},
		"slop_score_divisor":   {risk.SlopScoreDivisor, 10},
	} {
		assertIntSetting(t, name, want.got, want.want)
	}
}

func TestExplicitZeroRiskWeightRoundTripsThroughWrite(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "codeguard.yaml")
	if err := os.WriteFile(src, []byte(explicitZeroYAML), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := codeguard.LoadConfigFile(src)
	if err != nil {
		t.Fatalf("LoadConfigFile: %v", err)
	}
	// Written beside the source: loaded paths are absolute within dir, and
	// config artifact paths must stay inside the config's own directory.
	out := filepath.Join(dir, "written.yaml")
	if err = codeguard.WriteConfigFile(out, cfg); err != nil {
		t.Fatalf("WriteConfigFile: %v", err)
	}
	data, err := os.ReadFile(out) //nolint:gosec // test-owned temp file
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	if !strings.Contains(string(data), "security_weight: 0") {
		t.Fatalf("written config dropped the explicit zero weight:\n%s", data)
	}
	reloaded, err := codeguard.LoadConfigFile(out)
	if err != nil {
		t.Fatalf("reload written config: %v", err)
	}
	assertIntSetting(t, "security_weight after round trip", reloaded.Checks.QualityRules.RiskScoring.SecurityWeight, 0)
}

func TestNegativeRiskWeightsStillRejected(t *testing.T) {
	body := strings.Replace(explicitZeroYAML, "security_weight: 0", "security_weight: -1", 1)
	path := filepath.Join(t.TempDir(), "codeguard.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := codeguard.LoadConfigFile(path)
	if err == nil || !strings.Contains(err.Error(), "quality_rules.risk_scoring.security_weight must be non-negative") {
		t.Fatalf("LoadConfigFile error = %v, want security_weight non-negative error", err)
	}
}

func loadConfigFixture(t *testing.T, name, body string) codeguard.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := codeguard.LoadConfigFile(path)
	if err != nil {
		t.Fatalf("LoadConfigFile(%s): %v", name, err)
	}
	return cfg
}

func assertIntSetting(t *testing.T, name string, got int, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %d, want %d", name, got, want)
	}
}
