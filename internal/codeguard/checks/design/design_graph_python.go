package design

// moduleGraphFromPython adapts the Python-specific import graph onto the
// generic module graph used by god-module and change-impact analysis.
func moduleGraphFromPython(src pythonImportGraph) *moduleGraph {
	graph := newModuleGraph("python")
	for _, module := range src.graph.Order {
		graph.addModule(module, src.graph.Nodes[module].Path)
	}
	addPythonGraphEdges(graph, src)
	addPythonGraphImports(graph, src)
	return graph
}

func addPythonGraphEdges(graph *moduleGraph, src pythonImportGraph) {
	for _, module := range src.graph.Order {
		for _, edge := range src.graph.Nodes[module].Edges {
			if edge.To == module {
				graph.addSelfEdge(module, edge.Line)
				continue
			}
			graph.addEdge(module, edge.To, edge.Line)
		}
	}
}

func addPythonGraphImports(graph *moduleGraph, src pythonImportGraph) {
	for _, node := range src.nodes {
		for _, statement := range node.statements {
			if len(statement.modules) > 0 {
				addPythonModuleImports(graph, src, node, statement)
				continue
			}
			targets := pythonFromImportTargets(statement, src.nodes)
			if len(targets) == 0 {
				graph.addImport(node.module, "", node.file, statement.from, statement.line)
				continue
			}
			for _, target := range targets {
				graph.addImport(node.module, target, node.file, statement.from, statement.line)
			}
		}
	}
}

func addPythonModuleImports(graph *moduleGraph, src pythonImportGraph, node pythonGraphNode, statement pythonImportStatement) {
	for _, imported := range statement.modules {
		resolved := ""
		if _, ok := src.nodes[imported]; ok {
			resolved = imported
		}
		graph.addImport(node.module, resolved, node.file, imported, statement.line)
	}
}
