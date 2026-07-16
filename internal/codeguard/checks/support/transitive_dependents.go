package support

import "sort"

func ReverseDependencyMap(graph DependencyGraph) map[string][]string {
	reverse := make(map[string][]string, len(graph.Nodes))
	for from, node := range graph.Nodes {
		for _, edge := range node.Edges {
			reverse[edge.To] = append(reverse[edge.To], from)
		}
	}
	return reverse
}

func TransitiveDependents(reverse map[string][]string, root string) []string {
	seen := map[string]bool{root: true}
	queue := []string{root}
	dependents := make([]string, 0)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, dependent := range reverse[current] {
			if seen[dependent] {
				continue
			}
			seen[dependent] = true
			dependents = append(dependents, dependent)
			queue = append(queue, dependent)
		}
	}
	sort.Strings(dependents)
	return dependents
}
