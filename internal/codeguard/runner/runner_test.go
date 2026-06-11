package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestRunPublishesPythonDependencyGraphArtifact(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.py"), "from app import service\n")
	writeTestFile(t, filepath.Join(root, "app", "__init__.py"), "")
	writeTestFile(t, filepath.Join(root, "app", "service.py"), "from . import shared\n")
	writeTestFile(t, filepath.Join(root, "app", "shared.py"), "")

	cacheEnabled := false
	report, err := Run(context.Background(), core.Config{
		Name: "artifact-test",
		Targets: []core.TargetConfig{{
			Name:        "python-target",
			Path:        root,
			Language:    "python",
			Entrypoints: []string{"main.py"},
		}},
		Checks: core.CheckConfig{
			Design: true,
		},
		Output: core.OutputConfig{Format: "json"},
		Cache: core.CacheConfig{
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

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
