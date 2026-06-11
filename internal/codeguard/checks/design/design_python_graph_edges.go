package design

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func pythonStatementEdges(statement pythonImportStatement, known map[string]pythonGraphNode) []support.DependencyEdge {
	if len(statement.modules) > 0 {
		return pythonModuleImportEdges(statement, known)
	}
	return pythonFromImportEdges(statement, known)
}

func pythonModuleImportEdges(statement pythonImportStatement, known map[string]pythonGraphNode) []support.DependencyEdge {
	edges := make([]support.DependencyEdge, 0, len(statement.modules))
	for _, module := range statement.modules {
		if _, ok := known[module]; !ok {
			continue
		}
		edges = append(edges, support.DependencyEdge{To: module, Line: statement.line})
	}
	return edges
}

func pythonFromImportEdges(statement pythonImportStatement, known map[string]pythonGraphNode) []support.DependencyEdge {
	targets := pythonFromImportTargets(statement, known)
	edges := make([]support.DependencyEdge, 0, len(targets))
	for _, target := range targets {
		edges = append(edges, support.DependencyEdge{To: target, Line: statement.line, Names: statement.names})
	}
	return edges
}

func pythonFromImportTargets(statement pythonImportStatement, known map[string]pythonGraphNode) []string {
	targets := make([]string, 0, len(statement.names)+1)
	for _, name := range statement.names {
		if name == "*" {
			continue
		}
		candidate := statement.from + "." + name
		if _, ok := known[candidate]; ok {
			targets = append(targets, candidate)
		}
	}
	if len(targets) == 0 {
		if _, ok := known[statement.from]; ok {
			targets = append(targets, statement.from)
		}
	}
	return targets
}

func importsPrivatePythonModule(module string, names []string) bool {
	for _, part := range strings.Split(module, ".") {
		if strings.HasPrefix(part, "_") {
			return true
		}
	}
	for _, name := range names {
		if strings.HasPrefix(name, "_") {
			return true
		}
	}
	return false
}
