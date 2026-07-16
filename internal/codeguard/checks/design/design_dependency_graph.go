package design

import (
	"sort"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

// moduleGraph is a language-neutral module import graph used for cycle,
// god-module, and change-impact analysis across languages.
type moduleGraph struct {
	language     string
	modules      map[string]*moduleGraphNode
	order        []string
	fileToModule map[string]string
}

type moduleGraphNode struct {
	module string
	file   string
	edges  []moduleGraphEdge
}

type moduleGraphEdge struct {
	to   string
	line int
}

func newModuleGraph(language string) *moduleGraph {
	return &moduleGraph{
		language:     language,
		modules:      make(map[string]*moduleGraphNode),
		fileToModule: make(map[string]string),
	}
}

func (g *moduleGraph) addModule(module string, file string) {
	if module == "" {
		return
	}
	if _, ok := g.modules[module]; !ok {
		g.modules[module] = &moduleGraphNode{module: module, file: file}
		g.order = append(g.order, module)
	}
	if file != "" {
		if _, ok := g.fileToModule[file]; !ok {
			g.fileToModule[file] = module
		}
	}
}

func (g *moduleGraph) addEdge(from string, to string, line int) {
	node, ok := g.modules[from]
	if !ok || from == to {
		return
	}
	if _, known := g.modules[to]; !known {
		return
	}
	for _, edge := range node.edges {
		if edge.to == to {
			return
		}
	}
	node.edges = append(node.edges, moduleGraphEdge{to: to, line: line})
}

func (g *moduleGraph) addSelfEdge(module string, line int) {
	node, ok := g.modules[module]
	if !ok {
		return
	}
	for _, edge := range node.edges {
		if edge.to == module {
			return
		}
	}
	node.edges = append(node.edges, moduleGraphEdge{to: module, line: line})
}

func (g *moduleGraph) sortedOrder() []string {
	order := append([]string(nil), g.order...)
	sort.Strings(order)
	return order
}

// fanCounts returns fan-out (distinct imports) and fan-in (distinct importers)
// per module.
func (g *moduleGraph) fanCounts() (map[string]int, map[string]int) {
	fanOut := make(map[string]int, len(g.modules))
	fanIn := make(map[string]int, len(g.modules))
	for module, node := range g.modules {
		for _, edge := range node.edges {
			if edge.to == module {
				continue
			}
			fanOut[module]++
			fanIn[edge.to]++
		}
	}
	return fanOut, fanIn
}

// transitiveDependents returns every module that reaches the given module
// through one or more import edges, sorted by name.
func (g *moduleGraph) transitiveDependents(module string) []string {
	reverse := make(map[string][]string, len(g.modules))
	for from, node := range g.modules {
		for _, edge := range node.edges {
			reverse[edge.to] = append(reverse[edge.to], from)
		}
	}
	return support.TransitiveDependents(reverse, module)
}
