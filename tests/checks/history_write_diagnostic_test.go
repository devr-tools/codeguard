package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// blockCachePath points cfg's cache under a regular file, so every history
// write next to the cache fails.
func blockCachePath(t *testing.T, dir string, cfg *codeguard.Config) {
	t.Helper()
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Cache.Path = filepath.Join(blocker, "cache.json")
}

// A score-history file that cannot be written must surface as a warn-level
// operational diagnostic on the section that published the score, instead of
// being silently dropped. The runner records score history after the
// sections finalize, so this also pins where the diagnostic lands.
func TestScoreHistoryWriteFailureSurfacesDiagnostic(t *testing.T) {
	cases := []struct {
		name    string
		history string
		section string
		config  func(t *testing.T, dir string) codeguard.Config
	}{
		{
			name:    "slop",
			history: "slop",
			section: "quality",
			config: func(t *testing.T, dir string) codeguard.Config {
				writeSlopFixture(t, dir)
				return slopHistoryTestConfig(dir, "quality-ai-history-write-failure")
			},
		},
		{
			name:    "performance",
			history: "performance-score",
			section: "performance",
			config: func(t *testing.T, dir string) codeguard.Config {
				writePerformanceScoreFixture(t, dir)
				return performanceScoreConfig(dir, "performance-history-write-failure")
			},
		},
		{
			name:    "legibility",
			history: "legibility",
			section: "context",
			config: func(t *testing.T, dir string) codeguard.Config {
				repo := filepath.Join(dir, "repo")
				if err := os.MkdirAll(repo, 0o755); err != nil {
					t.Fatal(err)
				}
				writeLegibleRepoFixture(t, repo)
				return legibilityHistoryConfig(t, repo, dir, "legibility-history-write-failure")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := tc.config(t, dir)
			blockCachePath(t, dir, &cfg)

			report, err := codeguard.Run(context.Background(), cfg)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			sections := historyWriteDiagnosticSections(t, report)
			if sections[tc.history] != tc.section {
				t.Fatalf("%s history-write diagnostic on section %q, want %q (all: %v)", tc.history, sections[tc.history], tc.section, sections)
			}
		})
	}
}

// historyWriteDiagnosticSections maps each history named by a
// scan.history-write diagnostic to the section carrying it, asserting the
// diagnostic's level and its effect on the section status along the way.
func historyWriteDiagnosticSections(t *testing.T, report codeguard.Report) map[string]string {
	t.Helper()
	sections := map[string]string{}
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
			sections[diagnostic.Metadata["history"]] = section.ID
		}
	}
	return sections
}
