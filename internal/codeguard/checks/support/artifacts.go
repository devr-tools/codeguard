package support

import "github.com/devr-tools/codeguard/internal/codeguard/core"

const ArtifactKindDependencyGraph = "dependency_graph"

func NewDependencyGraphArtifact(id string, language string, target string, graph DependencyGraph) core.Artifact {
	nodes := make([]core.DependencyGraphNode, 0, len(graph.Order))
	for _, nodeID := range graph.Order {
		node := graph.Nodes[nodeID]
		edges := make([]core.DependencyGraphEdge, 0, len(node.Edges))
		for _, edge := range node.Edges {
			edges = append(edges, core.DependencyGraphEdge{
				To:    edge.To,
				Line:  edge.Line,
				Names: append([]string(nil), edge.Names...),
			})
		}
		nodes = append(nodes, core.DependencyGraphNode{
			ID:       node.ID,
			Path:     node.Path,
			IsPublic: node.IsPublic,
			Edges:    edges,
		})
	}
	return core.Artifact{
		ID:       id,
		Kind:     ArtifactKindDependencyGraph,
		Language: language,
		Target:   target,
		DependencyGraph: &core.DependencyGraphArtifact{
			Order: append([]string(nil), graph.Order...),
			Nodes: nodes,
		},
	}
}
