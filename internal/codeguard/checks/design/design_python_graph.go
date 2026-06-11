package design

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type pythonModuleNode struct {
	module     string
	file       string
	isPublic   bool
	statements []pythonImportStatement
	edges      []pythonImportEdge
}

type pythonImportEdge struct {
	to    string
	line  int
	names []string
}

type pythonImportGraph struct {
	modules     map[string]pythonModuleNode
	moduleOrder []string
	entrypoints map[string]struct{}
}

func buildPythonImportGraph(env support.Context, target core.TargetConfig) pythonImportGraph {
	graph := pythonImportGraph{
		modules:     make(map[string]pythonModuleNode),
		entrypoints: pythonEntrypointModules(target.Entrypoints),
	}
	env.ScanTargetFiles(target, "design", func(rel string) bool {
		return strings.EqualFold(filepath.Ext(rel), ".py")
	}, func(file string, data []byte) []core.Finding {
		module := pythonModuleName(file)
		pkg := pythonPackageName(file)
		graph.modules[module] = pythonModuleNode{
			module:     module,
			file:       file,
			isPublic:   isPublicPythonModule(file, target),
			statements: pythonImportStatements(module, pkg, data),
		}
		return nil
	})
	graph.moduleOrder = make([]string, 0, len(graph.modules))
	for module := range graph.modules {
		graph.moduleOrder = append(graph.moduleOrder, module)
	}
	sort.Strings(graph.moduleOrder)
	for _, module := range graph.moduleOrder {
		node := graph.modules[module]
		node.edges = resolvePythonImportTargets(node, graph.modules)
		graph.modules[module] = node
	}
	return graph
}

func resolvePythonImportTargets(node pythonModuleNode, known map[string]pythonModuleNode) []pythonImportEdge {
	edges := make([]pythonImportEdge, 0, len(node.statements))
	seen := make(map[string]struct{})
	for _, statement := range node.statements {
		for _, edge := range pythonStatementEdges(statement, known) {
			key := fmt.Sprintf("%s:%d", edge.to, edge.line)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			edges = append(edges, edge)
		}
	}
	return edges
}

func directPythonBoundaryFindings(env support.Context, node pythonModuleNode, entrypoints map[string]struct{}) []core.Finding {
	if !node.isPublic {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, edge := range node.edges {
		if importsPrivatePythonModule(edge.to, edge.names) {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.python.public-imports-private",
				Level:   "fail",
				Path:    node.file,
				Line:    edge.line,
				Column:  1,
				Message: "public Python module imports a private module",
			}))
		}
		if _, ok := entrypoints[edge.to]; ok {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.python.public-imports-cli",
				Level:   "fail",
				Path:    node.file,
				Line:    edge.line,
				Column:  1,
				Message: "public Python module imports a CLI or entrypoint module",
			}))
		}
	}
	return findings
}

func transitivePythonEntrypointFindings(env support.Context, graph pythonImportGraph) []core.Finding {
	memo := make(map[string][]string, len(graph.modules))
	findings := make([]core.Finding, 0)
	for _, module := range graph.moduleOrder {
		node := graph.modules[module]
		if !node.isPublic {
			continue
		}
		for _, edge := range node.edges {
			if _, ok := graph.entrypoints[edge.to]; ok {
				continue
			}
			path := pythonReachesEntrypoint(edge.to, graph, memo, map[string]bool{module: true})
			if len(path) == 0 {
				continue
			}
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.python.public-depends-on-cli",
				Level:   "fail",
				Path:    node.file,
				Line:    edge.line,
				Column:  1,
				Message: fmt.Sprintf("public Python module depends on a CLI or entrypoint module through import graph: %s", strings.Join(path, " -> ")),
			}))
			break
		}
	}
	return findings
}

func pythonReachesEntrypoint(module string, graph pythonImportGraph, memo map[string][]string, visiting map[string]bool) []string {
	if cached, ok := memo[module]; ok {
		return cached
	}
	if _, ok := graph.entrypoints[module]; ok {
		memo[module] = []string{module}
		return memo[module]
	}
	if visiting[module] {
		return nil
	}
	visiting[module] = true
	for _, edge := range graph.modules[module].edges {
		path := pythonReachesEntrypoint(edge.to, graph, memo, visiting)
		if len(path) == 0 {
			continue
		}
		chain := append([]string{module}, path...)
		memo[module] = chain
		delete(visiting, module)
		return chain
	}
	delete(visiting, module)
	memo[module] = nil
	return nil
}

func pythonImportCycleFindings(env support.Context, graph pythonImportGraph) []core.Finding {
	components := pythonStronglyConnectedComponents(graph)
	findings := make([]core.Finding, 0, len(components))
	for _, component := range components {
		if len(component) == 1 {
			node := graph.modules[component[0]]
			selfCycle := false
			for _, edge := range node.edges {
				if edge.to == node.module {
					selfCycle = true
					break
				}
			}
			if !selfCycle {
				continue
			}
		}
		sort.Strings(component)
		node := graph.modules[component[0]]
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "design.python.import-cycle",
			Level:   "fail",
			Path:    node.file,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("Python module import cycle detected: %s", strings.Join(component, " <-> ")),
		}))
	}
	return findings
}

func pythonStronglyConnectedComponents(graph pythonImportGraph) [][]string {
	index := 0
	stack := make([]string, 0, len(graph.modules))
	indices := make(map[string]int, len(graph.modules))
	lowlink := make(map[string]int, len(graph.modules))
	onStack := make(map[string]bool, len(graph.modules))
	components := make([][]string, 0)
	var visit func(string)
	visit = func(module string) {
		index++
		indices[module] = index
		lowlink[module] = index
		stack = append(stack, module)
		onStack[module] = true
		for _, edge := range graph.modules[module].edges {
			if indices[edge.to] == 0 {
				visit(edge.to)
				if lowlink[edge.to] < lowlink[module] {
					lowlink[module] = lowlink[edge.to]
				}
				continue
			}
			if onStack[edge.to] && indices[edge.to] < lowlink[module] {
				lowlink[module] = indices[edge.to]
			}
		}
		if lowlink[module] != indices[module] {
			return
		}
		component := make([]string, 0)
		for {
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[last] = false
			component = append(component, last)
			if last == module {
				break
			}
		}
		components = append(components, component)
	}
	for _, module := range graph.moduleOrder {
		if indices[module] == 0 {
			visit(module)
		}
	}
	return components
}
