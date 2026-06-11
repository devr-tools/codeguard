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
			Design: true,
		},
		Output: codeguard.OutputConfig{Format: "json"},
		Cache: codeguard.CacheConfig{
			Enabled: &cacheEnabled,
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(report.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(report.Artifacts))
	}
	artifact := report.Artifacts[0]
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

func writeArtifactFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
