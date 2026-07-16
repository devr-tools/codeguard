package support

import (
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// GoPackageImportGraph captures a target-local Go package graph keyed by
// package directory relative to the target root.
type GoPackageImportGraph struct {
	Graph         DependencyGraph
	FileToPackage map[string]string
}

type pendingGoPackageEdge struct {
	from string
	to   string
	line int
}

// BuildGoPackageImportGraph parses non-test Go files in a target, resolves
// intra-module imports through go.mod, and returns a package-level import graph.
// Nil means the target has no go.mod module path or no in-repo packages.
func BuildGoPackageImportGraph(env Context, target core.TargetConfig) *GoPackageImportGraph {
	modulePrefix := GoModulePath(target.Path)
	if modulePrefix == "" {
		return nil
	}
	nodes := make(map[string]DependencyNode)
	fileToPackage := make(map[string]string)
	pending := make([]pendingGoPackageEdge, 0)
	env.VisitTargetFiles(target, isGoPackageGraphSourceFile, func(rel string, data []byte) {
		pkg := path.Dir(filepath.ToSlash(rel))
		if _, ok := nodes[pkg]; !ok {
			nodes[pkg] = DependencyNode{ID: pkg, Path: rel}
		}
		if _, ok := fileToPackage[rel]; !ok {
			fileToPackage[rel] = pkg
		}
		pending = append(pending, goPackageImportEdges(pkg, rel, data, modulePrefix)...)
	})
	if len(nodes) == 0 {
		return nil
	}
	attachGoPackageEdges(nodes, pending)
	return &GoPackageImportGraph{
		Graph:         NewDependencyGraph(nodes),
		FileToPackage: fileToPackage,
	}
}

func attachGoPackageEdges(nodes map[string]DependencyNode, pending []pendingGoPackageEdge) {
	seenEdges := make(map[string]map[string]bool, len(nodes))
	for _, edge := range pending {
		if !validGoPackageEdge(nodes, edge, seenEdges) {
			continue
		}
		node := nodes[edge.from]
		node.Edges = append(node.Edges, DependencyEdge{To: edge.to, Line: edge.line})
		nodes[edge.from] = node
	}
}

func validGoPackageEdge(nodes map[string]DependencyNode, edge pendingGoPackageEdge, seenEdges map[string]map[string]bool) bool {
	if edge.from == edge.to {
		return false
	}
	if _, ok := nodes[edge.from]; !ok {
		return false
	}
	if _, ok := nodes[edge.to]; !ok {
		return false
	}
	if seenEdges[edge.from] == nil {
		seenEdges[edge.from] = make(map[string]bool)
	}
	if seenEdges[edge.from][edge.to] {
		return false
	}
	seenEdges[edge.from][edge.to] = true
	return true
}

func isGoPackageGraphSourceFile(rel string) bool {
	return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go")
}

func goPackageImportEdges(pkg string, rel string, data []byte, modulePrefix string) []pendingGoPackageEdge {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, rel, data, parser.ImportsOnly)
	if err != nil {
		return nil
	}
	edges := make([]pendingGoPackageEdge, 0, len(parsed.Imports))
	for _, imp := range parsed.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		local := goLocalPackageDir(importPath, modulePrefix)
		if local == "" {
			continue
		}
		edges = append(edges, pendingGoPackageEdge{
			from: pkg,
			to:   local,
			line: fset.Position(imp.Pos()).Line,
		})
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
