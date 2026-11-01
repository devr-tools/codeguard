package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// A history file that cannot be written must surface as a warn-level
// operational diagnostic instead of being silently dropped.
func TestSlopHistoryWriteFailureSurfacesDiagnostic(t *testing.T) {
	dir := t.TempDir()
	writeSlopFixture(t, dir)
	cfg := slopHistoryTestConfig(dir, "quality-ai-history-write-failure")
	// Make the cache directory a regular file so every history write fails.
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Cache.Path = filepath.Join(blocker, "cache.json")

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	histories := map[string]bool{}
	for _, section := range report.Sections {
		for _, diagnostic := range section.Diagnostics {
			if diagnostic.ID != "scan.history-write" {
				continue
			}
			if diagnostic.Level != "warn" || !diagnostic.Operational {
				t.Fatalf("history diagnostic = %#v, want warn-level operational", diagnostic)
			}
			if section.Status == codeguard.StatusPass {
				t.Fatalf("section %s carrying %#v should not pass", section.ID, diagnostic)
			}
			histories[diagnostic.Metadata["history"]] = true
		}
	}
	if !histories["slop"] {
		t.Fatalf("expected a slop history-write diagnostic, got %v in %#v", histories, report.Sections)
	}
}
