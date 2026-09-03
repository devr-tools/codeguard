package codeguard_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func confidenceScanConfig(t testing.TB) codeguard.Config {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < 6; i++ {
		var src strings.Builder
		src.WriteString("package repo\n\n")
		src.WriteString(fmt.Sprintf("func Long%02d() int {\n", i))
		for line := 0; line < 12; line++ {
			src.WriteString(fmt.Sprintf("\tv%d := %d\n\t_ = v%d\n", line, line, line))
		}
		src.WriteString("\treturn 0\n}\n")
		path := filepath.Join(dir, fmt.Sprintf("file%02d.go", i))
		if err := os.WriteFile(path, []byte(src.String()), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}

	cacheDisabled := false
	cfg := codeguard.ExampleConfig()
	cfg.Name = "confidence-policy"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Cache.Enabled = &cacheDisabled
	cfg.Checks.QualityRules.MaxFunctionLines = 5
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	return cfg
}

// An omitted policy and an explicit permissive policy must be indistinguishable,
// which is what makes the default upgrade path a no-op.
func TestConfidencePolicyDefaultMatchesExplicitPermissive(t *testing.T) {
	cfg := confidenceScanConfig(t)
	omitted, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run without policy: %v", err)
	}
	if omitted.Summary.TotalFindings == 0 {
		t.Fatal("fixture produced no findings; the comparison would be vacuous")
	}

	cfg.Checks.MinConfidence = codeguard.ConfidencePolicyConfig{Default: codeguard.ConfidenceLow}
	explicit, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run with permissive policy: %v", err)
	}
	if !reflect.DeepEqual(omitted.Sections, explicit.Sections) {
		t.Fatal("an explicit low threshold changed the scan; the default must admit everything")
	}
}

// End-to-end proof that the threshold reaches every section through the real
// runner, and that what it removes is accounted for rather than lost.
func TestConfidencePolicyFiltersThroughRealScan(t *testing.T) {
	cfg := confidenceScanConfig(t)
	permissive, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("permissive run: %v", err)
	}

	cfg.Checks.MinConfidence = codeguard.ConfidencePolicyConfig{Default: codeguard.ConfidenceHigh}
	strict, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("strict run: %v", err)
	}

	if strict.Summary.TotalFindings >= permissive.Summary.TotalFindings {
		t.Fatalf("strict findings = %d, permissive = %d: a high threshold must remove low-confidence findings",
			strict.Summary.TotalFindings, permissive.Summary.TotalFindings)
	}

	filtered := 0
	for _, section := range strict.Sections {
		filtered += section.ConfidenceFilteredCount
	}
	removed := permissive.Summary.TotalFindings - strict.Summary.TotalFindings
	if filtered < removed {
		t.Fatalf("confidence-filtered count = %d, but %d findings disappeared", filtered, removed)
	}
}

// The policy is keyed on the section ids that appear in scan output. This
// pins that contract: a section whose id is not a known key could never be
// configured, and the divergence would be silent.
func TestReportedSectionIDsAreConfigurableKeys(t *testing.T) {
	report, err := codeguard.Run(context.Background(), confidenceScanConfig(t))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(report.Sections) == 0 {
		t.Fatal("scan reported no sections")
	}
	for _, section := range report.Sections {
		if !core.KnownSectionKey(section.ID) {
			t.Fatalf("section %q is reported by a scan but is not a configurable section key (known: %s)",
				section.ID, strings.Join(core.SectionKeys(), ", "))
		}
	}
}
