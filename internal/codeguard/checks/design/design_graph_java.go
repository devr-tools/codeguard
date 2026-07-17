package design

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	javaPackagePattern = regexp.MustCompile(`^\s*package\s+([\w.]+)\s*;`)
	javaImportPattern  = regexp.MustCompile(`^\s*import\s+(static\s+)?([\w.]+(?:\.\*)?)\s*;`)
)

type javaImportStatement struct {
	target   string
	wildcard bool
	static   bool
	line     int
}

func buildJavaImportGraph(env support.Context, target core.TargetConfig) *moduleGraph {
	graph := newModuleGraph("java")
	packageModules := make(map[string][]string)
	pending := make(map[string][]javaImportStatement)
	env.VisitTargetFiles(target, func(rel string) bool {
		return strings.HasSuffix(rel, ".java")
	}, func(rel string, data []byte) {
		pkg, imports := parseJavaFile(string(data))
		module := javaModuleName(pkg, rel)
		graph.addModule(module, rel)
		packageModules[pkg] = append(packageModules[pkg], module)
		pending[module] = append(pending[module], imports...)
	})
	for module, imports := range pending {
		addJavaImportEdges(graph, packageModules, module, graph.modules[module].file, imports)
	}
	return graph
}

func parseJavaFile(source string) (string, []javaImportStatement) {
	pkg := ""
	imports := make([]javaImportStatement, 0)
	for idx, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		if match := javaPackagePattern.FindStringSubmatch(line); len(match) == 2 && pkg == "" {
			pkg = match[1]
			continue
		}
		match := javaImportPattern.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		imports = append(imports, javaImportStatement{
			target:   strings.TrimSuffix(match[2], ".*"),
			wildcard: strings.HasSuffix(match[2], ".*"),
			static:   strings.TrimSpace(match[1]) != "",
			line:     idx + 1,
		})
	}
	return pkg, imports
}

func javaModuleName(pkg string, rel string) string {
	base := strings.TrimSuffix(filepath.Base(rel), ".java")
	if pkg == "" {
		return base
	}
	return pkg + "." + base
}

func addJavaImportEdges(graph *moduleGraph, packageModules map[string][]string, module string, file string, imports []javaImportStatement) {
	for _, statement := range imports {
		if statement.wildcard {
			resolved := ""
			for _, member := range packageModules[statement.target] {
				graph.addImport(module, member, file, statement.target+".*", statement.line)
				resolved = member
			}
			if resolved == "" {
				graph.addImport(module, "", file, statement.target+".*", statement.line)
			}
			continue
		}
		resolved := resolveJavaImport(graph, statement)
		graph.addImport(module, resolved, file, statement.target, statement.line)
	}
}

// resolveJavaImport matches the longest known module prefix so imports of
// nested classes or static members map to the declaring file.
func resolveJavaImport(graph *moduleGraph, statement javaImportStatement) string {
	for current := statement.target; current != ""; current = javaImportPrefix(current) {
		if _, ok := graph.modules[current]; ok {
			return current
		}
	}
	return ""
}

func javaImportPrefix(target string) string {
	if cut := strings.LastIndex(target, "."); cut >= 0 {
		return target[:cut]
	}
	return ""
}
