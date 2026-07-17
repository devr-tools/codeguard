package design

import (
	"go/parser"
	"go/token"
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
	modulePrefix := support.GoModulePath(target.Path)
	if modulePrefix == "" {
		return nil
	}
	graph := newModuleGraph("go")
	pending := make([]pendingGraphEdge, 0)
	env.VisitTargetFiles(target, isGoSourceFile, func(rel string, data []byte) {
		pkg := path.Dir(filepath.ToSlash(rel))
		graph.addModule(pkg, rel)
		pending = append(pending, goImportEdges(pkg, rel, data)...)
	})
	for _, edge := range pending {
		local := goLocalPackageDir(edge.to, modulePrefix)
		graph.addImport(edge.from, local, edge.file, edge.to, edge.line)
	}
	return graph
}

func isGoSourceFile(rel string) bool {
	return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go")
}

func goImportEdges(pkg string, rel string, data []byte) []pendingGraphEdge {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, rel, data, parser.ImportsOnly)
	if err != nil {
		return nil
	}
	edges := make([]pendingGraphEdge, 0, len(parsed.Imports))
	for _, imp := range parsed.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		edges = append(edges, pendingGraphEdge{from: pkg, to: importPath, file: rel, line: fset.Position(imp.Pos()).Line})
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
