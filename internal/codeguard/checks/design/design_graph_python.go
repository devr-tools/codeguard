package design

// moduleGraphFromPython adapts the Python-specific import graph onto the
// generic module graph used by god-module and change-impact analysis.
func moduleGraphFromPython(src pythonImportGraph) *moduleGraph {
	graph := newModuleGraph("python")
	for _, module := range src.moduleOrder {
		graph.addModule(module, src.modules[module].file)
	}
	for _, module := range src.moduleOrder {
		for _, edge := range src.modules[module].edges {
			if edge.to == module {
				graph.addSelfEdge(module, edge.line)
				continue
			}
			graph.addEdge(module, edge.to, edge.line)
		}
	}
	return graph
}
