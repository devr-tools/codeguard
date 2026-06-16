package design

// stronglyConnectedComponents runs Tarjan's algorithm over the module graph
// and returns the strongly connected components in discovery order.
func (g *moduleGraph) stronglyConnectedComponents() [][]string {
	state := &tarjanState{
		graph:   g,
		indices: make(map[string]int, len(g.modules)),
		lowlink: make(map[string]int, len(g.modules)),
		onStack: make(map[string]bool, len(g.modules)),
	}
	for _, module := range g.sortedOrder() {
		if state.indices[module] == 0 {
			state.visit(module)
		}
	}
	return state.components
}

type tarjanState struct {
	graph      *moduleGraph
	index      int
	stack      []string
	indices    map[string]int
	lowlink    map[string]int
	onStack    map[string]bool
	components [][]string
}

func (s *tarjanState) visit(module string) {
	s.index++
	s.indices[module] = s.index
	s.lowlink[module] = s.index
	s.stack = append(s.stack, module)
	s.onStack[module] = true
	for _, edge := range s.graph.modules[module].edges {
		if s.indices[edge.to] == 0 {
			s.visit(edge.to)
			if s.lowlink[edge.to] < s.lowlink[module] {
				s.lowlink[module] = s.lowlink[edge.to]
			}
			continue
		}
		if s.onStack[edge.to] && s.indices[edge.to] < s.lowlink[module] {
			s.lowlink[module] = s.indices[edge.to]
		}
	}
	if s.lowlink[module] != s.indices[module] {
		return
	}
	component := make([]string, 0)
	for {
		last := s.stack[len(s.stack)-1]
		s.stack = s.stack[:len(s.stack)-1]
		s.onStack[last] = false
		component = append(component, last)
		if last == module {
			break
		}
	}
	s.components = append(s.components, component)
}
