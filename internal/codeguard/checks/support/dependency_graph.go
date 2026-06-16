package support

import "sort"

type DependencyNode struct {
	ID       string
	Path     string
	IsPublic bool
	Edges    []DependencyEdge
}

type DependencyEdge struct {
	To    string
	Line  int
	Names []string
}

type DependencyGraph struct {
	Nodes map[string]DependencyNode
	Order []string
}

func NewDependencyGraph(nodes map[string]DependencyNode) DependencyGraph {
	order := make([]string, 0, len(nodes))
	for id := range nodes {
		order = append(order, id)
	}
	sort.Strings(order)
	return DependencyGraph{
		Nodes: nodes,
		Order: order,
	}
}

func (graph DependencyGraph) ReachablePath(start string, target func(string) bool) []string {
	return graph.reachablePath(start, target, make(map[string][]string, len(graph.Nodes)), map[string]bool{})
}

func (graph DependencyGraph) reachablePath(start string, target func(string) bool, memo map[string][]string, visiting map[string]bool) []string {
	if cached, ok := memo[start]; ok {
		return cached
	}
	if target(start) {
		memo[start] = []string{start}
		return memo[start]
	}
	if visiting[start] {
		return nil
	}
	visiting[start] = true
	node, ok := graph.Nodes[start]
	if !ok {
		delete(visiting, start)
		memo[start] = nil
		return nil
	}
	for _, edge := range node.Edges {
		path := graph.reachablePath(edge.To, target, memo, visiting)
		if len(path) == 0 {
			continue
		}
		chain := append([]string{start}, path...)
		memo[start] = chain
		delete(visiting, start)
		return chain
	}
	delete(visiting, start)
	memo[start] = nil
	return nil
}

func (graph DependencyGraph) StronglyConnectedComponents() [][]string {
	state := newTarjanState(graph)
	for _, id := range graph.Order {
		if state.indices[id] == 0 {
			state.visit(id)
		}
	}
	return state.components
}
