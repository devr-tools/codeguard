package support

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/cpp/compdb"
)

var (
	cppModuleDeclarationPattern = regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?module[ \t]+([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*(?::[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)?)[ \t]*;`)
	cppModuleImportPattern      = regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?import[ \t]+((?:[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*(?::[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)?)|(?::[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*))[ \t]*;`)
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
	declaredModules := make(map[string]string)
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
			declaredModules[rel] = match[1]
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
	includeRoots := cppTargetIncludeRoots(env, target)
	for _, dependency := range pending {
		to := ""
		if dependency.named {
			dependency.target = qualifyCPPModuleImport(dependency.target, declaredModules[dependency.from])
			to = moduleFiles[dependency.target]
		} else {
			to = resolveCPPInclude(nodes, dependency.from, dependency.target, includeRoots)
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

func qualifyCPPModuleImport(specifier string, declaredModule string) string {
	if !strings.HasPrefix(specifier, ":") {
		return specifier
	}
	primary := cppPrimaryModuleName(declaredModule)
	if primary == "" {
		return ""
	}
	return primary + specifier
}

func cppPrimaryModuleName(module string) string {
	module = strings.TrimSpace(module)
	if module == "" {
		return ""
	}
	if cut := strings.IndexByte(module, ':'); cut >= 0 {
		return module[:cut]
	}
	return module
}

func resolveCPPInclude(nodes map[string]DependencyNode, from string, imported string, includeRoots []string) string {
	imported = filepath.ToSlash(imported)
	candidates := make([]string, 0, 2+len(includeRoots))
	candidates = append(candidates,
		path.Clean(path.Join(path.Dir(from), imported)),
		path.Clean(imported),
	)
	for _, root := range includeRoots {
		candidates = append(candidates, path.Clean(path.Join(root, imported)))
	}
	resolved := ""
	for _, candidate := range candidates {
		if _, ok := nodes[candidate]; ok {
			if resolved != "" && resolved != candidate {
				return ""
			}
			resolved = candidate
		}
	}
	return resolved
}

func cppTargetIncludeRoots(env Context, target core.TargetConfig) []string {
	db, err := compdb.Load(target.Path, env.Config.Checks.QualityRules.CPPTooling.CompileCommands)
	if err != nil {
		return nil
	}
	root, err := filepath.Abs(target.Path)
	if err != nil {
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, entry := range db.Entries {
		for _, include := range entry.IncludeDirs {
			rel, err := filepath.Rel(root, include)
			if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			rel = filepath.ToSlash(rel)
			if !seen[rel] {
				seen[rel] = true
				result = append(result, rel)
			}
		}
	}
	return result
}
