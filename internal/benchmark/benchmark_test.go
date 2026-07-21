package benchmark

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validManifest() Manifest {
	return Manifest{Version: SchemaVersion, Corpus: "frozen-prs-v1", Entries: []Entry{{
		ID: "typescript-pr-2", Language: "typescript", Repository: "github.com/example/web", PullRequest: 2,
		BaseRevision: "aaaaaaaa", HeadRevision: "bbbbbbbb", Worktree: "typescript-pr-2", Config: ".codeguard/codeguard.yaml",
	}}}
}

func TestManifestExportIsStableAndDoesNotExposePaths(t *testing.T) {
	manifest := validManifest()
	export := manifest.Export()
	encoded, err := json.Marshal(export)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || string(encoded) == `{"version":1}` {
		t.Fatalf("unexpected export %s", encoded)
	}
	if strings.Contains(string(encoded), "worktree") || strings.Contains(string(encoded), "config") {
		t.Fatalf("export leaked local fields: %s", encoded)
	}
}

func TestManifestRejectsUnsafeRelativePaths(t *testing.T) {
	manifest := validManifest()
	manifest.Entries[0].Worktree = "../outside"
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected unsafe worktree error")
	}
}

// The checked-in template is CI-validated by the ordinary Go test job. A
// future frozen manifest can use the same schema without adding a bespoke CI
// workflow or requiring network access.
func TestCheckedInManifestExamplesValidate(t *testing.T) {
	if _, err := Load(filepath.Join("..", "..", "benchmarks", "manifest.example.json")); err != nil {
		t.Fatalf("checked-in benchmark manifest must validate: %v", err)
	}
}

func TestRunRecordsColdAndWarmMeasurements(t *testing.T) {
	root := t.TempDir()
	entry := validManifest().Entries[0]
	worktree := filepath.Join(root, entry.Worktree)
	if err := os.MkdirAll(filepath.Join(worktree, ".codeguard"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, entry.Config), []byte("name: benchmark\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := validManifest()
	result, err := Run(context.Background(), manifest, RunOptions{Binary: "/usr/bin/true", WorkRoot: root, WarmRepeats: 2, Now: func() time.Time { return time.Unix(0, 0) }})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Runs) != 3 || result.Runs[0].Mode != "cold" || result.Runs[2].Attempt != 2 {
		t.Fatalf("unexpected runs: %#v", result.Runs)
	}
}
