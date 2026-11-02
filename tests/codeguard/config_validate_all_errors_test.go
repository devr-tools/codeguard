package codeguard_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// ValidateConfig used to stop at the first failing validator, so a config with
// three mistakes took three edit-and-rerun loops to fix. It now reports every
// failure, keeping the first one as the leading line for existing matchers.
func TestValidateConfigReportsEveryFailingValidator(t *testing.T) {
	bad := codeguard.ExampleConfig()
	bad.Output.Format = "bogus"
	bad.Baseline.Governance.MaxEntries = -1
	bad.Checks.QualityRules.Naming.RoleSuffixWarnThreshold = -1

	outputOnly := codeguard.ExampleConfig()
	outputOnly.Output.Format = "bogus"
	firstErr := codeguard.ValidateConfig(outputOnly)
	if firstErr == nil {
		t.Fatal("ValidateConfig accepted output.format=bogus")
	}

	err := codeguard.ValidateConfig(bad)
	if err == nil {
		t.Fatal("ValidateConfig accepted a config with three invalid settings")
	}
	lines := strings.Split(err.Error(), "\n")
	if lines[0] != firstErr.Error() {
		t.Fatalf("leading line = %q, want the first validator's message %q", lines[0], firstErr.Error())
	}
	for _, want := range []string{
		"baseline.governance.max_entries must not be negative",
		"quality_rules.naming.role_suffix_warn_threshold must not be negative",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error is missing %q; got:\n%s", want, err)
		}
	}
	if len(lines) != 3 {
		t.Errorf("error has %d lines, want 3 (one per invalid setting):\n%s", len(lines), err)
	}
}

func TestValidateConfigSingleErrorIsUnchanged(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.Baseline.Governance.SampleLimit = -1
	err := codeguard.ValidateConfig(cfg)
	if err == nil || err.Error() != "baseline.governance.sample_limit must not be negative" {
		t.Fatalf("error = %v, want exactly the sample_limit message", err)
	}
	if err := codeguard.ValidateConfig(codeguard.ExampleConfig()); err != nil {
		t.Fatalf("example config should validate cleanly, got %v", err)
	}
}
