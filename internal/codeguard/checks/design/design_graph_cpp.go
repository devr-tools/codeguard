package design

import (
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

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
	return graph
}
