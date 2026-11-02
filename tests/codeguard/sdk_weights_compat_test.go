package codeguard_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
	"gopkg.in/yaml.v3"
)

// The 13 risk weights are plain ints in the 1.x SDK. This file pins that Go
// code written against 1.10 still compiles and behaves the same (0 means
// "use the default"), and that the explicit-zero opt-in works for SDK callers.

type weightCase struct {
	key  string
	def  int
	get  func(codeguard.Config) int
	set  func(*codeguard.Config, int)
	mark func(*codeguard.Config, ...string) error
}

func riskWeight(key string, def int, field func(*codeguard.RiskScoringConfig) *int) weightCase {
	return weightCase{
		key: "checks.quality_rules.risk_scoring." + key,
		def: def,
		get: func(cfg codeguard.Config) int { return *field(&cfg.Checks.QualityRules.RiskScoring) },
		set: func(cfg *codeguard.Config, v int) { *field(&cfg.Checks.QualityRules.RiskScoring) = v },
		mark: func(cfg *codeguard.Config, keys ...string) error {
			return cfg.Checks.QualityRules.RiskScoring.SetExplicit(keys...)
		},
	}
}

func productionWeight(key string, def int, field func(*codeguard.ProductionRiskConfig) *int) weightCase {
	return weightCase{
		key: "checks.production_risk." + key,
		def: def,
		get: func(cfg codeguard.Config) int { return *field(&cfg.Checks.ProductionRisk) },
		set: func(cfg *codeguard.Config, v int) { *field(&cfg.Checks.ProductionRisk) = v },
		mark: func(cfg *codeguard.Config, keys ...string) error {
			return cfg.Checks.ProductionRisk.SetExplicit(keys...)
		},
	}
}

func allWeightCases() []weightCase {
	return []weightCase{
		riskWeight("changed_file_weight", 5, func(c *codeguard.RiskScoringConfig) *int { return &c.ChangedFileWeight }),
		riskWeight("fail_finding_weight", 30, func(c *codeguard.RiskScoringConfig) *int { return &c.FailFindingWeight }),
		riskWeight("warn_finding_weight", 15, func(c *codeguard.RiskScoringConfig) *int { return &c.WarnFindingWeight }),
		riskWeight("security_weight", 10, func(c *codeguard.RiskScoringConfig) *int { return &c.SecurityWeight }),
		riskWeight("supply_chain_weight", 10, func(c *codeguard.RiskScoringConfig) *int { return &c.SupplyChainWeight }),
		riskWeight("coverage_gap_weight", 15, func(c *codeguard.RiskScoringConfig) *int { return &c.CoverageGapWeight }),
		riskWeight("ai_provenance_weight", 15, func(c *codeguard.RiskScoringConfig) *int { return &c.AIProvenanceWeight }),
		riskWeight("ai_signal_weight", 10, func(c *codeguard.RiskScoringConfig) *int { return &c.AISignalWeight }),
		riskWeight("slop_score_divisor", 10, func(c *codeguard.RiskScoringConfig) *int { return &c.SlopScoreDivisor }),
		productionWeight("reliability_weight", 12, func(c *codeguard.ProductionRiskConfig) *int { return &c.ReliabilityWeight }),
		productionWeight("data_weight", 15, func(c *codeguard.ProductionRiskConfig) *int { return &c.DataWeight }),
		productionWeight("fail_weight", 25, func(c *codeguard.ProductionRiskConfig) *int { return &c.FailWeight }),
		productionWeight("warn_weight", 10, func(c *codeguard.ProductionRiskConfig) *int { return &c.WarnWeight }),
	}
}

func (c weightCase) leaf() string { return c.key[strings.LastIndex(c.key, ".")+1:] }

// TestSDK110StyleWeightAssignmentsCompile is the compile-level guard: this is
// how 1.10 SDK code set the weights, an int assigned to each field. If any
// field stops being an int, this file stops compiling.
func TestSDK110StyleWeightAssignmentsCompile(t *testing.T) {
	var cfg codeguard.Config
	cfg.Checks.QualityRules.RiskScoring.ChangedFileWeight = 1
	cfg.Checks.QualityRules.RiskScoring.FailFindingWeight = 2
	cfg.Checks.QualityRules.RiskScoring.WarnFindingWeight = 3
	cfg.Checks.QualityRules.RiskScoring.SecurityWeight = 4
	cfg.Checks.QualityRules.RiskScoring.SupplyChainWeight = 5
	cfg.Checks.QualityRules.RiskScoring.CoverageGapWeight = 6
	cfg.Checks.QualityRules.RiskScoring.AIProvenanceWeight = 7
	cfg.Checks.QualityRules.RiskScoring.AISignalWeight = 8
	cfg.Checks.QualityRules.RiskScoring.SlopScoreDivisor = 9
	cfg.Checks.ProductionRisk = codeguard.ProductionRiskConfig{ReliabilityWeight: 10, DataWeight: 11, FailWeight: 12, WarnWeight: 13}
	// A []int literal only accepts int operands, so this pins the field type
	// exactly (an untyped constant assignment above would also fit float64).
	risk := cfg.Checks.QualityRules.RiskScoring
	prod := cfg.Checks.ProductionRisk
	asInts := []int{
		risk.ChangedFileWeight, risk.FailFindingWeight, risk.WarnFindingWeight, risk.SecurityWeight,
		risk.SupplyChainWeight, risk.CoverageGapWeight, risk.AIProvenanceWeight, risk.AISignalWeight,
		risk.SlopScoreDivisor, prod.ReliabilityWeight, prod.DataWeight, prod.FailWeight, prod.WarnWeight,
	}

	codeguard.ApplyDefaults(&cfg)
	if len(asInts) != 13 || asInts[12] != 13 {
		t.Fatalf("weights read as ints = %v, want 1..13", asInts)
	}
	for idx, tc := range allWeightCases() {
		if got := tc.get(cfg); got != idx+1 {
			t.Errorf("%s = %d after ApplyDefaults, want the assigned %d", tc.key, got, idx+1)
		}
	}
}

// TestSDKZeroWeightStillMeansDefault pins the 1.10 semantics: a Go caller
// that leaves a weight 0 (or assigns 0) gets the default.
func TestSDKZeroWeightStillMeansDefault(t *testing.T) {
	for _, tc := range allWeightCases() {
		var cfg codeguard.Config
		tc.set(&cfg, 0)
		codeguard.ApplyDefaults(&cfg)
		if got := tc.get(cfg); got != tc.def {
			t.Errorf("%s = %d for an unmarked 0, want default %d", tc.key, got, tc.def)
		}
	}
}

func TestSDKSetExplicitKeepsZero(t *testing.T) {
	for _, tc := range allWeightCases() {
		var cfg codeguard.Config
		tc.set(&cfg, 0)
		if err := tc.mark(&cfg, tc.leaf()); err != nil {
			t.Fatalf("SetExplicit(%q): %v", tc.leaf(), err)
		}
		codeguard.ApplyDefaults(&cfg)
		if got := tc.get(cfg); got != 0 {
			t.Errorf("%s = %d after SetExplicit, want 0", tc.key, got)
		}
		if err := codeguard.ValidateConfig(cfg); err != nil && strings.Contains(err.Error(), tc.leaf()) {
			t.Errorf("explicit 0 for %s rejected: %v", tc.key, err)
		}
	}
}

func TestSDKSetExplicitAcceptsGoFieldNamesAndRejectsUnknownKeys(t *testing.T) {
	var risk codeguard.RiskScoringConfig
	if err := risk.SetExplicit("SecurityWeight", "slop_score_divisor"); err != nil {
		t.Fatalf("SetExplicit: %v", err)
	}
	if !risk.IsExplicit("security_weight") || !risk.IsExplicit("SlopScoreDivisor") {
		t.Fatalf("explicit marks not recorded for both name forms")
	}
	err := risk.SetExplicit("warn_finding_weight", "max_hotspots")
	if err == nil || !strings.Contains(err.Error(), "max_hotspots") {
		t.Fatalf("SetExplicit(max_hotspots) error = %v, want unsupported-key error", err)
	}
	if risk.IsExplicit("warn_finding_weight") {
		t.Fatalf("a rejected SetExplicit call must not mark any key")
	}
	if got := len(risk.ExplicitKeys()); got != 9 {
		t.Fatalf("RiskScoringConfig.ExplicitKeys() has %d keys, want 9", got)
	}
	var prod codeguard.ProductionRiskConfig
	if got := len(prod.ExplicitKeys()); got != 4 {
		t.Fatalf("ProductionRiskConfig.ExplicitKeys() has %d keys, want 4", got)
	}
	if err := prod.SetExplicit("security_weight"); err == nil {
		t.Fatalf("ProductionRiskConfig accepted a risk-scoring key")
	}
}

func TestSDKClearExplicitRestoresDefault(t *testing.T) {
	cfg := loadConfigFixture(t, "codeguard.yaml", explicitZeroYAML)
	risk := cfg.Checks.QualityRules.RiskScoring
	if !risk.IsExplicit("security_weight") {
		t.Fatalf("security_weight from the file is not explicit")
	}
	if err := risk.ClearExplicit("security_weight"); err != nil {
		t.Fatalf("ClearExplicit: %v", err)
	}
	cfg.Checks.QualityRules.RiskScoring = risk
	codeguard.ApplyDefaults(&cfg)
	assertIntSetting(t, "security_weight after ClearExplicit", cfg.Checks.QualityRules.RiskScoring.SecurityWeight, 10)
	assertIntSetting(t, "slop_score_divisor untouched", cfg.Checks.QualityRules.RiskScoring.SlopScoreDivisor, 0)
}

// TestEveryWeightKeyExplicitZeroInFiles covers all 13 keys in YAML and JSON:
// written as 0 it stays 0; absent it takes the default.
func TestEveryWeightKeyExplicitZeroInFiles(t *testing.T) {
	for _, tc := range allWeightCases() {
		for _, format := range []string{"yaml", "json"} {
			cfg := loadConfigFixture(t, "codeguard."+format, singleKeyConfig(format, tc.key, "0"))
			if got := tc.get(cfg); got != 0 {
				t.Errorf("%s: %s: 0 = %d after load, want 0", format, tc.key, got)
			}
			absent := loadConfigFixture(t, "codeguard."+format, singleKeyConfig(format, "", ""))
			if got := tc.get(absent); got != tc.def {
				t.Errorf("%s: %s absent = %d, want default %d", format, tc.key, got, tc.def)
			}
		}
	}
}

func TestNullWeightInFileTakesDefault(t *testing.T) {
	for _, format := range []string{"yaml", "json"} {
		cfg := loadConfigFixture(t, "codeguard."+format, singleKeyConfig(format, "checks.production_risk.fail_weight", "null"))
		assertIntSetting(t, format+" fail_weight: null", cfg.Checks.ProductionRisk.FailWeight, 25)
	}
}

func TestYAMLAnchorMergeKeepsExplicitZero(t *testing.T) {
	body := `name: zero
targets:
  - name: app
    path: .
    language: go
base_weights: &zeros
  security_weight: 0
checks:
  quality_rules:
    risk_scoring:
      <<: *zeros
      warn_finding_weight: 2
`
	// base_weights is not a config key; the decoder ignores it, and the
	// anchored mapping is merged into risk_scoring with `<<`.
	cfg := loadConfigFixture(t, "codeguard.yaml", body)
	assertIntSetting(t, "merged security_weight", cfg.Checks.QualityRules.RiskScoring.SecurityWeight, 0)
	assertIntSetting(t, "warn_finding_weight", cfg.Checks.QualityRules.RiskScoring.WarnFindingWeight, 2)
}

// Explicit marks must ride along wherever a config is copied, merged field by
// field, re-profiled, or defaulted again; losing one silently turns a
// deliberate 0 back into the default weight.
func TestExplicitMarksSurviveCopiesAndRepeatedDefaults(t *testing.T) {
	loaded := loadConfigFixture(t, "codeguard.yaml", explicitZeroYAML)

	copied := loaded
	copied.Profile = "strict"
	codeguard.ApplyDefaults(&copied)
	codeguard.ApplyDefaults(&copied)
	assertIntSetting(t, "security_weight after profile + defaults", copied.Checks.QualityRules.RiskScoring.SecurityWeight, 0)
	assertIntSetting(t, "production fail_weight after profile + defaults", copied.Checks.ProductionRisk.FailWeight, 0)

	var merged codeguard.Config
	merged.Checks.QualityRules = loaded.Checks.QualityRules
	merged.Checks.ProductionRisk = loaded.Checks.ProductionRisk
	codeguard.ApplyDefaults(&merged)
	assertIntSetting(t, "security_weight after field merge", merged.Checks.QualityRules.RiskScoring.SecurityWeight, 0)
	assertIntSetting(t, "production fail_weight after field merge", merged.Checks.ProductionRisk.FailWeight, 0)
	assertIntSetting(t, "unset warn_weight after field merge", merged.Checks.ProductionRisk.WarnWeight, 10)
}

func TestProfileInFileKeepsExplicitZero(t *testing.T) {
	for _, profile := range []string{"startup", "strict", "enterprise", "ai-safe"} {
		body := strings.Replace(explicitZeroYAML, "name: zero\n", "name: zero\nprofile: "+profile+"\n", 1)
		cfg := loadConfigFixture(t, "codeguard.yaml", body)
		assertIntSetting(t, profile+" security_weight", cfg.Checks.QualityRules.RiskScoring.SecurityWeight, 0)
		assertIntSetting(t, profile+" production fail_weight", cfg.Checks.ProductionRisk.FailWeight, 0)
		assertIntSetting(t, profile+" warn_finding_weight default", cfg.Checks.QualityRules.RiskScoring.WarnFindingWeight, 15)
	}
}

func TestExplicitZeroEncodesInJSONAndYAML(t *testing.T) {
	var cfg codeguard.Config
	if err := cfg.Checks.ProductionRisk.SetExplicit("fail_weight"); err != nil {
		t.Fatalf("SetExplicit: %v", err)
	}
	if err := cfg.Checks.QualityRules.RiskScoring.SetExplicit("security_weight"); err != nil {
		t.Fatalf("SetExplicit: %v", err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"fail_weight":0`) || !strings.Contains(string(data), `"security_weight":0`) {
		t.Fatalf("JSON dropped explicit zeros: %s", data)
	}
	var back codeguard.Config
	if err = json.Unmarshal(data, &back); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if !back.Checks.ProductionRisk.IsExplicit("fail_weight") || !back.Checks.QualityRules.RiskScoring.IsExplicit("security_weight") {
		t.Fatalf("JSON round trip lost explicit marks")
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if !strings.Contains(string(out), "fail_weight: 0") || !strings.Contains(string(out), "security_weight: 0") {
		t.Fatalf("YAML dropped explicit zeros:\n%s", out)
	}
	var backYAML codeguard.Config
	if err = yaml.Unmarshal(out, &backYAML); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if !backYAML.Checks.ProductionRisk.IsExplicit("fail_weight") {
		t.Fatalf("YAML round trip lost explicit marks")
	}

	// Unmarked zeros stay omitted, as in 1.10.
	plain, err := json.Marshal(codeguard.Config{})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(plain), "fail_weight") {
		t.Fatalf("unmarked zero weight encoded: %s", plain)
	}
}

// singleKeyConfig renders a minimal config that sets one dotted key (or none
// when path is empty) to the raw value, in YAML or JSON.
func singleKeyConfig(format, path, raw string) string {
	segments := []string{}
	if path != "" {
		segments = strings.Split(path, ".")
	}
	if format == "json" {
		var b strings.Builder
		b.WriteString(`{"name": "one", "targets": [{"name": "app", "path": ".", "language": "go"}]`)
		for i, segment := range segments {
			if i == 0 {
				b.WriteString(", ")
			}
			b.WriteString(`"` + segment + `": `)
			if i < len(segments)-1 {
				b.WriteString("{")
			}
		}
		if len(segments) > 0 {
			b.WriteString(raw)
			b.WriteString(strings.Repeat("}", len(segments)-1))
		}
		b.WriteString("}")
		return b.String()
	}
	var b strings.Builder
	b.WriteString("name: one\ntargets:\n  - name: app\n    path: .\n    language: go\n")
	for i, segment := range segments {
		b.WriteString(strings.Repeat("  ", i))
		b.WriteString(segment)
		b.WriteString(":")
		if i == len(segments)-1 {
			b.WriteString(" ")
			b.WriteString(raw)
		}
		b.WriteString("\n")
	}
	return b.String()
}
