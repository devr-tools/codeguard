package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -sarif-output lets one scan feed both the -format output on stdout and a
// SARIF file for a code scanning upload. The file is written even when the
// scan fails, and the exit status still reflects the findings.
func TestScanSARIFOutputWritesFileAlongsideFormat(t *testing.T) {
	configPath := ciFixture(t, "fail")
	sarifPath := filepath.Join(t.TempDir(), "nested", "codeguard.sarif")
	code, stdout, stderr := runCLI(t, "scan", "-quiet", "-format", "github", "-sarif-output", sarifPath, "-config", configPath)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 for a failing scan; stderr = %s", code, stderr)
	}
	if !strings.Contains(stdout, "::error") {
		t.Fatalf("stdout lost the -format github annotations:\n%s", stdout)
	}
	raw, err := os.ReadFile(sarifPath)
	if err != nil {
		t.Fatalf("read SARIF output: %v", err)
	}
	var doc struct {
		Version string `json:"version"`
		Runs    []struct {
			Results []struct {
				RuleID string `json:"ruleId"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("SARIF output is not JSON: %v\n%s", err, raw)
	}
	if doc.Version != "2.1.0" || len(doc.Runs) != 1 {
		t.Fatalf("SARIF output version %q with %d runs, want 2.1.0 with one run", doc.Version, len(doc.Runs))
	}
	found := false
	for _, result := range doc.Runs[0].Results {
		found = found || result.RuleID == "quality.gofmt"
	}
	if !found {
		t.Fatalf("SARIF output lacks the failing quality.gofmt result:\n%s", raw)
	}
}

// A scan that passes its exit gate writes the SARIF file too, so an upload
// step keeps code scanning alerts current whatever the gate decided.
func TestScanSARIFOutputOnPassingScan(t *testing.T) {
	configPath := ciFixture(t, "warn")
	sarifPath := filepath.Join(t.TempDir(), "codeguard.sarif")
	if code, _, stderr := runCLI(t, "scan", "-quiet", "-fail-on", "none", "-sarif-output", sarifPath, "-config", configPath); code != 0 {
		t.Fatalf("exit = %d, want 0; stderr = %s", code, stderr)
	}
	if _, err := os.Stat(sarifPath); err != nil {
		t.Fatalf("SARIF output missing after a passing scan: %v", err)
	}
}

// An unwritable -sarif-output is an operational failure, not a silent skip.
func TestScanSARIFOutputUnwritablePathFails(t *testing.T) {
	configPath := ciFixture(t, "warn")
	blocker := filepath.Join(t.TempDir(), "file")
	writeScanTestFile(t, blocker, "not a directory")
	code, _, stderr := runCLI(t, "scan", "-quiet", "-fail-on", "none", "-sarif-output", filepath.Join(blocker, "out.sarif"), "-config", configPath)
	if code == 0 || !strings.Contains(stderr, "SARIF output") {
		t.Fatalf("exit = %d, stderr = %s; want a non-zero exit naming the SARIF output", code, stderr)
	}
}
