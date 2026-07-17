package design

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	cppBoundaryModuleDeclarationPattern = regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?module[ \t]+([A-Za-z_]\w*(?::[A-Za-z_]\w*)*)[ \t]*;`)
	cppBoundaryModuleImportPattern      = regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?import[ \t]+([A-Za-z_]\w*(?::[A-Za-z_]\w*)*)[ \t]*;`)
)

type cppBoundaryNamedImport struct {
	from      string
	specifier string
	line      int
}

func buildCPPImportGraph(env support.Context, target core.TargetConfig) *moduleGraph {
	dependencyGraph := support.BuildCPPDependencyGraph(env, target)
	if dependencyGraph == nil {
		return nil
	}
	graph := newModuleGraph("cpp")
	for _, id := range dependencyGraph.Graph.Order {
		node := dependencyGraph.Graph.Nodes[id]
		graph.addModule(id, node.Path)
	}
	for _, id := range dependencyGraph.Graph.Order {
		for _, edge := range dependencyGraph.Graph.Nodes[id].Edges {
			graph.addEdge(id, edge.To, edge.Line)
		}
	}
	namedModules := make(map[string]string)
	namedImports := make([]cppBoundaryNamedImport, 0)
	env.VisitTargetFiles(target, func(rel string) bool { return support.IsCPPPath(rel, true) }, func(rel string, data []byte) {
		parsed := support.ParseCLike(string(data), support.CLikeCPP)
		for _, imported := range parsed.Imports {
			resolved := cppGraphEdgeAtLine(graph, rel, imported.Line)
			if resolved == "" {
				resolved = resolveCPPGraphImport(graph, rel, imported.Module)
			}
			graph.addImport(rel, resolved, rel, imported.Module, imported.Line)
		}
		if match := cppBoundaryModuleDeclarationPattern.FindStringSubmatch(parsed.Masked); len(match) == 2 {
			namedModules[match[1]] = rel
		}
		for _, match := range cppBoundaryModuleImportPattern.FindAllStringSubmatchIndex(parsed.Masked, -1) {
			specifier := strings.TrimSpace(parsed.Masked[match[2]:match[3]])
			namedImports = append(namedImports, cppBoundaryNamedImport{from: rel, specifier: specifier, line: support.LineNumberForOffset(parsed.Masked, match[0])})
		}
	})
	for _, imported := range namedImports {
		graph.addImport(imported.from, namedModules[imported.specifier], imported.from, imported.specifier, imported.line)
	}
	return graph
}

func cppGraphEdgeAtLine(graph *moduleGraph, from string, line int) string {
	node := graph.modules[from]
	if node == nil {
		return ""
	}
	for _, edge := range node.edges {
		if edge.line == line {
			return edge.to
		}
	}
	return ""
}

func resolveCPPGraphImport(graph *moduleGraph, from string, specifier string) string {
	for _, candidate := range []string{
		pathCleanJoin(from, specifier),
		strings.TrimPrefix(strings.ReplaceAll(specifier, "\\", "/"), "./"),
	} {
		if _, ok := graph.modules[candidate]; ok {
			return candidate
		}
	}
	return ""
}

func pathCleanJoin(from string, specifier string) string {
	from = strings.ReplaceAll(from, "\\", "/")
	parts := strings.Split(from, "/")
	if len(parts) > 0 {
		parts = parts[:len(parts)-1]
	}
	joined := append(parts, strings.Split(strings.ReplaceAll(specifier, "\\", "/"), "/")...)
	stack := make([]string, 0, len(joined))
	for _, part := range joined {
		switch part {
		case "", ".":
		case "..":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		default:
			stack = append(stack, part)
		}
	}
	return strings.Join(stack, "/")
}
