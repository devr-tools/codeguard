package codeguard_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestCyclomaticCountingValidation(t *testing.T) {
	for _, mode := range []string{"", "legacy", "gocyclo", "GoCyclo", " legacy "} {
		cfg := codeguard.ExampleConfig()
		cfg.Checks.QualityRules.CyclomaticCounting = mode
		if err := codeguard.ValidateConfig(cfg); err != nil {
			t.Errorf("cyclomatic_counting=%q rejected: %v", mode, err)
		}
	}
	for _, tc := range []struct {
		mode    string
		wantMsg []string
	}{
		{"gocylo", []string{"quality_rules.cyclomatic_counting", `"gocylo"`, `did you mean "gocyclo"?`}},
		{"legasy", []string{"quality_rules.cyclomatic_counting", `did you mean "legacy"?`}},
		{"mccabe", []string{"quality_rules.cyclomatic_counting must be one of legacy, gocyclo", `"mccabe"`}},
	} {
		cfg := codeguard.ExampleConfig()
		cfg.Checks.QualityRules.CyclomaticCounting = tc.mode
		err := codeguard.ValidateConfig(cfg)
		if err == nil {
			t.Errorf("cyclomatic_counting=%q accepted, want a validation error", tc.mode)
			continue
		}
		for _, want := range tc.wantMsg {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("cyclomatic_counting=%q error = %q, want it to contain %q", tc.mode, err, want)
			}
		}
	}
}

// codeguard's own self-scan opts into the recommended gocyclo counting so the
// refactors made against it stay enforced while the 1.x default is legacy.
func TestSelfScanConfigOptsIntoGocycloCounting(t *testing.T) {
	cfg, err := codeguard.LoadConfigFile(filepath.Join("..", "..", ".codeguard", "codeguard.yaml"))
	if err != nil {
		t.Fatalf("load self-scan config: %v", err)
	}
	if got := cfg.Checks.QualityRules.CyclomaticCounting; got != "gocyclo" {
		t.Fatalf("self-scan quality_rules.cyclomatic_counting = %q, want gocyclo", got)
	}
}
