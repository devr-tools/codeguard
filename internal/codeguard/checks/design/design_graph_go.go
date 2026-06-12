package design

import (
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// buildGoImportGraph builds a package-level import graph for a Go target.
// Packages are identified by their directory relative to the target root and
// imports are resolved through the go.mod module path.
func buildGoImportGraph(env support.Context, target core.TargetConfig) *moduleGraph {
	modulePrefix := goModulePrefix(target.Path)
	if modulePrefix == "" {
		return nil
	}
	graph := newModuleGraph("go")
	pending := make([]pendingGraphEdge, 0)
	env.VisitTargetFiles(target, isGoSourceFile, func(rel string, data []byte) {
		pkg := path.Dir(filepath.ToSlash(rel))
		graph.addModule(pkg, rel)
		pending = append(pending, goImportEdges(pkg, rel, data, modulePrefix)...)
	})
	for _, edge := range pending {
		graph.addEdge(edge.from, edge.to, edge.line)
	}
	return graph
}

func isGoSourceFile(rel string) bool {
	return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go")
}

func goImportEdges(pkg string, rel string, data []byte, modulePrefix string) []pendingGraphEdge {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, rel, data, parser.ImportsOnly)
	if err != nil {
		return nil
	}
	edges := make([]pendingGraphEdge, 0, len(parsed.Imports))
	for _, imp := range parsed.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		local := goLocalPackageDir(importPath, modulePrefix)
		if local == "" {
			continue
		}
		edges = append(edges, pendingGraphEdge{from: pkg, to: local, line: fset.Position(imp.Pos()).Line})
	}
	return edges
}

func goLocalPackageDir(importPath string, modulePrefix string) string {
	if importPath == modulePrefix {
		return "."
	}
	if strings.HasPrefix(importPath, modulePrefix+"/") {
		return strings.TrimPrefix(importPath, modulePrefix+"/")
	}
	return ""
}

func goModulePrefix(targetPath string) string {
	data, err := os.ReadFile(filepath.Join(targetPath, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
