package codeguard_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestRunPublishesPythonDependencyGraphArtifact(t *testing.T) {
	root := t.TempDir()
	writeArtifactFile(t, filepath.Join(root, "main.py"), "from app import service\n")
	writeArtifactFile(t, filepath.Join(root, "app", "__init__.py"), "")
	writeArtifactFile(t, filepath.Join(root, "app", "service.py"), "from . import shared\n")
	writeArtifactFile(t, filepath.Join(root, "app", "shared.py"), "")

	cacheEnabled := false
	report, err := codeguard.Run(context.Background(), codeguard.Config{
		Name: "artifact-test",
		Targets: []codeguard.TargetConfig{{
			Name:        "python-target",
			Path:        root,
			Language:    "python",
			Entrypoints: []string{"main.py"},
		}},
		Checks: codeguard.CheckConfig{
			Design:  true,
			Context: contextOff(),
		},
		Output: codeguard.OutputConfig{Format: "json"},
		Cache: codeguard.CacheConfig{
			Enabled: &cacheEnabled,
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertPythonDependencyGraphArtifact(t, report, root)
}

func TestArtifactStoreListSortsAndReplaces(t *testing.T) {
	store := runnersupport.NewArtifactStore()
	store.Put(core.Artifact{ID: "b", Kind: "dependency_graph", Language: "python"})
	store.Put(core.Artifact{ID: "a", Kind: "dependency_graph", Language: "go"})
	store.Put(core.Artifact{ID: "b", Kind: "dependency_graph", Language: "typescript"})

	artifacts := store.List()
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(artifacts))
	}
	if artifacts[0].ID != "a" || artifacts[1].ID != "b" {
		t.Fatalf("expected sorted artifact IDs [a b], got [%s %s]", artifacts[0].ID, artifacts[1].ID)
	}
	if artifacts[1].Language != "typescript" {
		t.Fatalf("expected replacement artifact language typescript, got %q", artifacts[1].Language)
	}
}

func TestRunPublishesAISlopScoreArtifact(t *testing.T) {
	root := t.TempDir()
	writeArtifactFile(t, filepath.Join(root, "service.go"), `package sample

// Initialize the client.
func buildClient() error {
	err := doThing()
	_ = err
	return nil
}

func doThing() error { return nil }
`)

	cacheEnabled := false
	report, err := codeguard.Run(context.Background(), codeguard.Config{
		Name: "slop-score-test",
		Targets: []codeguard.TargetConfig{{
			Name:     "go-target",
			Path:     root,
			Language: "go",
		}},
		Checks: codeguard.CheckConfig{
			Quality: true,
			Context: contextOff(),
		},
		Output: codeguard.OutputConfig{Format: "json"},
		Cache: codeguard.CacheConfig{
			Enabled: &cacheEnabled,
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertSlopScoreArtifact(t, report, root)
	assertChangeRiskArtifact(t, report, root)
}

func writeArtifactFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func assertPythonDependencyGraphArtifact(t *testing.T, report codeguard.Report, root string) {
	t.Helper()
	if len(report.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(report.Artifacts))
	}
	artifact := report.Artifacts[0]
	assertArtifactMetadata(t, artifact, root)
	assertDependencyGraphPayload(t, artifact)
}

func assertArtifactMetadata(t *testing.T, artifact core.Artifact, root string) {
	t.Helper()
	if artifact.ID != "dependency_graph.python.python-target" {
		t.Fatalf("unexpected artifact ID %q", artifact.ID)
	}
	if artifact.Kind != "dependency_graph" {
		t.Fatalf("unexpected artifact kind %q", artifact.Kind)
	}
	if artifact.Language != "python" {
		t.Fatalf("unexpected artifact language %q", artifact.Language)
	}
	if artifact.Target != root {
		t.Fatalf("unexpected artifact target %q", artifact.Target)
	}
}

func assertDependencyGraphPayload(t *testing.T, artifact core.Artifact) {
	t.Helper()
	if artifact.DependencyGraph == nil {
		t.Fatal("expected dependency graph payload")
	}
	if len(artifact.DependencyGraph.Nodes) != 4 {
		t.Fatalf("expected 4 dependency graph nodes, got %d", len(artifact.DependencyGraph.Nodes))
	}
	if len(artifact.DependencyGraph.Order) != 4 {
		t.Fatalf("expected 4 dependency graph order entries, got %d", len(artifact.DependencyGraph.Order))
	}
	if artifact.DependencyGraph.Nodes[0].ID != "app" {
		t.Fatalf("expected sorted first node app, got %q", artifact.DependencyGraph.Nodes[0].ID)
	}
}

func assertSlopScoreArtifact(t *testing.T, report codeguard.Report, root string) {
	t.Helper()
	for _, artifact := range report.Artifacts {
		if artifact.Kind != "slop_score" {
			continue
		}
		if artifact.Target != root {
			t.Fatalf("unexpected slop artifact target %q", artifact.Target)
		}
		if artifact.SlopScore == nil {
			t.Fatal("expected slop score payload")
		}
		if artifact.SlopScore.Score <= 0 || artifact.SlopScore.Signals <= 0 {
			t.Fatalf("unexpected slop score payload %#v", artifact.SlopScore)
		}
		return
	}
	t.Fatalf("expected slop_score artifact, got %#v", report.Artifacts)
}

func assertChangeRiskArtifact(t *testing.T, report codeguard.Report, root string) {
	t.Helper()
	for _, artifact := range report.Artifacts {
		if artifact.Kind != "change_risk" {
			continue
		}
		if artifact.Target != root {
			t.Fatalf("unexpected change-risk artifact target %q", artifact.Target)
		}
		if artifact.ChangeRisk == nil {
			t.Fatal("expected change risk payload")
		}
		if artifact.ChangeRisk.Score <= 0 {
			t.Fatalf("unexpected change risk payload %#v", artifact.ChangeRisk)
		}
		return
	}
	t.Fatalf("expected change_risk artifact, got %#v", report.Artifacts)
}
