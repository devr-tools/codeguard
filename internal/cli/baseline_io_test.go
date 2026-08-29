package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestPruneWritesOnlyActiveEntriesAndNeverAddsFindings(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "baseline.json")
	output := filepath.Join(dir, "candidate.json")
	original := core.BaselineFile{GeneratedAt: "old", Entries: []core.BaselineEntry{
		{Fingerprint: "active", RuleID: "quality.a"},
		{Fingerprint: "stale", RuleID: "quality.b"},
	}}
	writeFixture(t, source, original)
	result := Audit(original, []core.Finding{{Fingerprint: "active", RuleID: "quality.a"}, {Fingerprint: "new", RuleID: "security.new"}}, Options{})

	if err := WritePruned(source, output, result, PruneOptions{}); err != nil {
		t.Fatalf("WritePruned: %v", err)
	}
	got, err := Load(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 1 || got.Entries[0].Fingerprint != "active" {
		t.Fatalf("pruned entries = %#v", got.Entries)
	}
	sourceAfter, err := os.ReadFile(source) //nolint:gosec // source is created inside t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	var unchanged core.BaselineFile
	if err := json.Unmarshal(sourceAfter, &unchanged); err != nil || len(unchanged.Entries) != 2 {
		t.Fatalf("source was modified: %s err=%v", sourceAfter, err)
	}
}

func TestWritePrunedRefusesInvalidEntries(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "baseline.json")
	file := core.BaselineFile{Entries: []core.BaselineEntry{{Fingerprint: ""}}}
	writeFixture(t, source, file)
	result := Audit(file, nil, Options{})
	if err := WritePruned(source, source, result, PruneOptions{}); err == nil {
		t.Fatal("expected invalid baseline refusal")
	}
}

func TestWritePrunedPreservesAllEntriesInAContextCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	file := core.BaselineFile{Entries: []core.BaselineEntry{
		{Fingerprint: "old-a", ContextFingerprint: "shared", RuleID: "quality.duplicate"},
		{Fingerprint: "old-b", ContextFingerprint: "shared", RuleID: "quality.duplicate"},
	}}
	writeFixture(t, path, file)
	result := Audit(file, []core.Finding{
		{Fingerprint: "current-a", ContextFingerprint: "shared", RuleID: "quality.duplicate"},
		{Fingerprint: "current-b", ContextFingerprint: "shared", RuleID: "quality.duplicate"},
	}, Options{})
	if err := WritePruned(path, path, result, PruneOptions{}); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("collision entries = %#v", got.Entries)
	}
}

func writeFixture(t *testing.T, path string, file core.BaselineFile) {
	t.Helper()
	data, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
