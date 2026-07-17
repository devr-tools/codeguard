package design

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type pythonImportGraph struct {
	graph       support.DependencyGraph
	entrypoints map[string]struct{}
	nodes       map[string]pythonGraphNode
}

type pythonGraphNode struct {
	module     string
	file       string
	isPublic   bool
	statements []pythonImportStatement
}

func buildPythonImportGraph(env support.Context, target core.TargetConfig) pythonImportGraph {
	graph := pythonImportGraph{
		entrypoints: pythonEntrypointModules(target.Entrypoints),
	}
	nodes := make(map[string]pythonGraphNode)
	// VisitTargetFiles rather than ScanTargetFiles: the visitor builds
	// cross-file state (the nodes map), so it must run sequentially and must
	// observe every file even when the per-file findings cache is warm.
	env.VisitTargetFiles(target, func(rel string) bool {
		return strings.EqualFold(filepath.Ext(rel), ".py")
	}, func(file string, data []byte) {
		module := pythonModuleName(file)
		pkg := pythonPackageName(file)
		nodes[module] = pythonGraphNode{
			module:     module,
			file:       file,
			isPublic:   isPublicPythonModule(file, target),
			statements: pythonImportStatements(module, pkg, data),
		}
	})
	dependencyNodes := make(map[string]support.DependencyNode, len(nodes))
	for module, node := range nodes {
		dependencyNodes[module] = support.DependencyNode{
			ID:       module,
			Path:     node.file,
			IsPublic: node.isPublic,
			Edges:    resolvePythonImportTargets(node, nodes),
		}
	}
	graph.graph = support.NewDependencyGraph(dependencyNodes)
	graph.nodes = nodes
	if env.PutArtifact != nil {
		env.PutArtifact(support.NewDependencyGraphArtifact(pythonDependencyGraphArtifactID(target), "python", target.Path, graph.graph))
	}
	return graph
}

func pythonDependencyGraphArtifactID(target core.TargetConfig) string {
	name := strings.TrimSpace(target.Name)
	if name == "" {
		name = strings.TrimSpace(target.Path)
	}
	return "dependency_graph.python." + name
}

func resolvePythonImportTargets(node pythonGraphNode, known map[string]pythonGraphNode) []support.DependencyEdge {
	edges := make([]support.DependencyEdge, 0, len(node.statements))
	seen := make(map[string]struct{})
	for _, statement := range node.statements {
		for _, edge := range pythonStatementEdges(statement, known) {
			key := fmt.Sprintf("%s:%d", edge.To, edge.Line)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			edges = append(edges, edge)
		}
	}
	return edges
}

func directPythonBoundaryFindings(env support.Context, node support.DependencyNode, entrypoints map[string]struct{}) []core.Finding {
	if !node.IsPublic {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, edge := range node.Edges {
		if importsPrivatePythonModule(edge.To, edge.Names) {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.python.public-imports-private",
				Level:   "fail",
				Path:    node.Path,
				Line:    edge.Line,
				Column:  1,
				Message: "public Python module imports a private module",
			}))
		}
		if _, ok := entrypoints[edge.To]; ok {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.python.public-imports-cli",
				Level:   "fail",
				Path:    node.Path,
				Line:    edge.Line,
				Column:  1,
				Message: "public Python module imports a CLI or entrypoint module",
			}))
		}
	}
	return findings
}

func transitivePythonEntrypointFindings(env support.Context, graph pythonImportGraph) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, module := range graph.graph.Order {
		node := graph.graph.Nodes[module]
		if !node.IsPublic {
			continue
		}
		for _, edge := range node.Edges {
			if _, ok := graph.entrypoints[edge.To]; ok {
				continue
			}
			path := graph.graph.ReachablePath(edge.To, func(id string) bool {
				_, ok := graph.entrypoints[id]
				return ok
			})
			if len(path) == 0 {
				continue
			}
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.python.public-depends-on-cli",
				Level:   "fail",
				Path:    node.Path,
				Line:    edge.Line,
				Column:  1,
				Message: fmt.Sprintf("public Python module depends on a CLI or entrypoint module through import graph: %s", strings.Join(path, " -> ")),
			}))
			break
		}
	}
	return findings
}

func pythonImportCycleFindings(env support.Context, graph pythonImportGraph) []core.Finding {
	components := graph.graph.StronglyConnectedComponents()
	findings := make([]core.Finding, 0, len(components))
	for _, component := range components {
		if len(component) == 1 {
			node := graph.graph.Nodes[component[0]]
			selfCycle := false
			for _, edge := range node.Edges {
				if edge.To == node.ID {
					selfCycle = true
					break
				}
			}
			if !selfCycle {
				continue
			}
		}
		sort.Strings(component)
		node := graph.graph.Nodes[component[0]]
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "design.python.import-cycle",
			Level:   "fail",
			Path:    node.Path,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("Python module import cycle detected: %s", strings.Join(component, " <-> ")),
		}))
	}
	return findings
}
