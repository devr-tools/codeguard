package support

type tarjanState struct {
	graph      DependencyGraph
	index      int
	stack      []string
	indices    map[string]int
	lowlink    map[string]int
	onStack    map[string]bool
	components [][]string
}

func newTarjanState(graph DependencyGraph) *tarjanState {
	return &tarjanState{
		graph:      graph,
		stack:      make([]string, 0, len(graph.Nodes)),
		indices:    make(map[string]int, len(graph.Nodes)),
		lowlink:    make(map[string]int, len(graph.Nodes)),
		onStack:    make(map[string]bool, len(graph.Nodes)),
		components: make([][]string, 0),
	}
}

func (state *tarjanState) visit(id string) {
	state.push(id)
	for _, edge := range state.graph.Nodes[id].Edges {
		state.visitEdge(id, edge.To)
	}
	if state.lowlink[id] == state.indices[id] {
		state.components = append(state.components, state.popComponent(id))
	}
}

func (state *tarjanState) push(id string) {
	state.index++
	state.indices[id] = state.index
	state.lowlink[id] = state.index
	state.stack = append(state.stack, id)
	state.onStack[id] = true
}

func (state *tarjanState) visitEdge(from string, to string) {
	if state.indices[to] == 0 {
		state.visit(to)
		state.lowlink[from] = minInt(state.lowlink[from], state.lowlink[to])
		return
	}
	if state.onStack[to] {
		state.lowlink[from] = minInt(state.lowlink[from], state.indices[to])
	}
}

func (state *tarjanState) popComponent(root string) []string {
	component := make([]string, 0)
	for {
		last := state.stack[len(state.stack)-1]
		state.stack = state.stack[:len(state.stack)-1]
		state.onStack[last] = false
		component = append(component, last)
		if last == root {
			return component
		}
	}
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
