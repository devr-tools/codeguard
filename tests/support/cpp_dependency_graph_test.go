package support_test

import (
	"os"
	"path/filepath"
	"testing"

	checksupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestCPPDependencyGraphUsesTargetLocalCompilationDatabaseIncludes(t *testing.T) {
	root := t.TempDir()
	writeCPPGraphFile(t, root, "src/main.cpp", "#include <project/widget.h>\n")
	writeCPPGraphFile(t, root, "include/project/widget.h", "#pragma once\n")
	writeCPPGraphFile(t, root, "compile_commands.json", `[{"directory":".","file":"src/main.cpp","arguments":["clang++","-Iinclude","-c","src/main.cpp"]}]`)
	target := core.TargetConfig{Path: root, Language: "cpp"}
	env := checksupport.Context{
		Config: core.Config{Checks: core.CheckConfig{QualityRules: core.QualityRulesConfig{CPPTooling: core.CPPToolingConfig{}}}},
		VisitTargetFiles: func(_ core.TargetConfig, include func(string) bool, visit func(string, []byte)) {
			for _, rel := range []string{"src/main.cpp", "include/project/widget.h"} {
				if !include(rel) {
					continue
				}
				data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
				if err != nil {
					t.Fatal(err)
				}
				visit(rel, data)
			}
		},
	}
	graph := checksupport.BuildCPPDependencyGraph(env, target)
	if graph == nil {
		t.Fatal("expected graph")
	}
	edges := graph.Graph.Nodes["src/main.cpp"].Edges
	if len(edges) != 1 || edges[0].To != "include/project/widget.h" {
		t.Fatalf("edges = %#v", edges)
	}
}

func writeCPPGraphFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // path is rooted in t.TempDir
		t.Fatal(err)
	}
}
