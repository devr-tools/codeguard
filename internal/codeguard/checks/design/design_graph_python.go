package design

// moduleGraphFromPython adapts the Python-specific import graph onto the
// generic module graph used by god-module and change-impact analysis.
func moduleGraphFromPython(src pythonImportGraph) *moduleGraph {
	graph := newModuleGraph("python")
	for _, module := range src.graph.Order {
		graph.addModule(module, src.graph.Nodes[module].Path)
	}
	for _, module := range src.graph.Order {
		for _, edge := range src.graph.Nodes[module].Edges {
			if edge.To == module {
				graph.addSelfEdge(module, edge.Line)
				continue
			}
			graph.addEdge(module, edge.To, edge.Line)
		}
	}
	return graph
}
