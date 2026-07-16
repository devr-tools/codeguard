package support

import (
	"path"
	"path/filepath"
	"regexp"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	cppModuleDeclarationPattern = regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?module[ \t]+([A-Za-z_]\w*(?::[A-Za-z_]\w*)*)[ \t]*;`)
	cppModuleImportPattern      = regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?import[ \t]+([A-Za-z_]\w*(?::[A-Za-z_]\w*)*)[ \t]*;`)
)

// CPPDependencyGraph captures target-local #include and C++20 named-module
// dependencies. Nodes are files because headers, unlike Go packages, are the
// unit whose edits fan out compilation work.
type CPPDependencyGraph struct {
	Graph        DependencyGraph
	FileToModule map[string]string
}

type pendingCPPDependency struct {
	from   string
	target string
	line   int
	named  bool
}

// BuildCPPDependencyGraph resolves only includes and module imports that map
// unambiguously to files inside the target. Compiler include paths and system
// headers are intentionally ignored because they cannot be inferred safely.
func BuildCPPDependencyGraph(env Context, target core.TargetConfig) *CPPDependencyGraph {
	nodes := make(map[string]DependencyNode)
	fileToModule := make(map[string]string)
	moduleFiles := make(map[string]string)
	pending := make([]pendingCPPDependency, 0)
	env.VisitTargetFiles(target, func(rel string) bool { return IsCPPPath(rel, true) }, func(rel string, data []byte) {
		rel = filepath.ToSlash(rel)
		nodes[rel] = DependencyNode{ID: rel, Path: rel}
		fileToModule[rel] = rel
		source := string(data)
		parsed := ParseCLike(source, CLikeCPP)
		for _, imported := range parsed.Imports {
			pending = append(pending, pendingCPPDependency{from: rel, target: imported.Module, line: imported.Line})
		}
		if match := cppModuleDeclarationPattern.FindStringSubmatch(parsed.Masked); match != nil {
			moduleFiles[match[1]] = rel
		}
		for _, match := range cppModuleImportPattern.FindAllStringSubmatchIndex(parsed.Masked, -1) {
			pending = append(pending, pendingCPPDependency{
				from: rel, target: parsed.Masked[match[2]:match[3]],
				line: LineNumberForOffset(parsed.Masked, match[0]), named: true,
			})
		}
	})
	if len(nodes) == 0 {
		return nil
	}
	seen := make(map[string]map[string]bool, len(nodes))
	for _, dependency := range pending {
		to := ""
		if dependency.named {
			to = moduleFiles[dependency.target]
		} else {
			to = resolveCPPInclude(nodes, dependency.from, dependency.target)
		}
		if to == "" || to == dependency.from {
			continue
		}
		if seen[dependency.from] == nil {
			seen[dependency.from] = make(map[string]bool)
		}
		if seen[dependency.from][to] {
			continue
		}
		seen[dependency.from][to] = true
		node := nodes[dependency.from]
		node.Edges = append(node.Edges, DependencyEdge{To: to, Line: dependency.line})
		nodes[dependency.from] = node
	}
	return &CPPDependencyGraph{Graph: NewDependencyGraph(nodes), FileToModule: fileToModule}
}

func resolveCPPInclude(nodes map[string]DependencyNode, from string, imported string) string {
	imported = filepath.ToSlash(imported)
	candidates := []string{
		path.Clean(path.Join(path.Dir(from), imported)),
		path.Clean(imported),
	}
	for _, candidate := range candidates {
		if _, ok := nodes[candidate]; ok {
			return candidate
		}
	}
	return ""
}
