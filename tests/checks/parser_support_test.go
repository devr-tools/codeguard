package checks_test

import (
	"testing"

	supportpkg "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func TestDependencyGraphReachablePath(t *testing.T) {
	graph := supportpkg.NewDependencyGraph(map[string]supportpkg.DependencyNode{
		"app.service": {
			ID: "app.service",
			Edges: []supportpkg.DependencyEdge{
				{To: "app.web"},
			},
		},
		"app.web": {
			ID: "app.web",
			Edges: []supportpkg.DependencyEdge{
				{To: "app.cli"},
			},
		},
		"app.cli": {ID: "app.cli"},
	})

	path := graph.ReachablePath("app.service", func(id string) bool {
		return id == "app.cli"
	})
	if len(path) != 3 {
		t.Fatalf("path length = %d, want 3 (%v)", len(path), path)
	}
	if path[0] != "app.service" || path[1] != "app.web" || path[2] != "app.cli" {
		t.Fatalf("path = %v, want app.service -> app.web -> app.cli", path)
	}
}

func TestDependencyGraphStronglyConnectedComponents(t *testing.T) {
	graph := supportpkg.NewDependencyGraph(map[string]supportpkg.DependencyNode{
		"app.repo": {
			ID: "app.repo",
			Edges: []supportpkg.DependencyEdge{
				{To: "app.service"},
			},
		},
		"app.service": {
			ID: "app.service",
			Edges: []supportpkg.DependencyEdge{
				{To: "app.repo"},
			},
		},
	})

	components := graph.StronglyConnectedComponents()
	if len(components) != 1 {
		t.Fatalf("component count = %d, want 1", len(components))
	}
	if len(components[0]) != 2 {
		t.Fatalf("component size = %d, want 2 (%v)", len(components[0]), components[0])
	}
}
