package version_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomebrewVersionExtractorReadsCompiledDefault(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	cmd := exec.Command("bash", filepath.Join("scripts", "version-from-source.sh"))
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("extract Homebrew version: %v\n%s", err, output)
	}
	if got, want := strings.TrimSpace(string(output)), "0.1.0"; got != want {
		t.Fatalf("extracted version = %q, want %q", got, want)
	}
}
