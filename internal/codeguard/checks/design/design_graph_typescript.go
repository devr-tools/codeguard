package design

import (
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var typeScriptImportSpecifierPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^[ \t]*import\s+[^'"\n]*?from\s+['"]([^'"]+)['"]`),
	regexp.MustCompile(`(?m)^[ \t]*import\s+['"]([^'"]+)['"]`),
	regexp.MustCompile(`(?m)^[ \t]*export\s+[^'"\n]*?from\s+['"]([^'"]+)['"]`),
	regexp.MustCompile(`\brequire\(\s*['"]([^'"]+)['"]\s*\)`),
	regexp.MustCompile(`\bimport\(\s*['"]([^'"]+)['"]\s*\)`),
}

var typeScriptModuleExtensions = []string{".d.ts", ".tsx", ".ts", ".jsx", ".js", ".mjs", ".cjs", ".mts", ".cts"}

type scriptImport struct {
	specifier string
	line      int
}

type pendingGraphEdge struct {
	from string
	to   string
	file string
	line int
}

func buildTypeScriptImportGraph(env support.Context, target core.TargetConfig) *moduleGraph {
	graph := newModuleGraph("typescript")
	pending := make([]pendingGraphEdge, 0)
	env.VisitTargetFiles(target, isTypeScriptLikeFile, func(rel string, data []byte) {
		module := typeScriptModuleKey(rel)
		graph.addModule(module, rel)
		source := strings.ReplaceAll(string(data), "\r\n", "\n")
		for _, imp := range typeScriptImportSpecifiers(source) {
			pending = append(pending, pendingGraphEdge{from: module, to: imp.specifier, file: rel, line: imp.line})
		}
	})
	for _, edge := range pending {
		resolved := resolveTypeScriptImport(graph, edge.from, edge.to)
		graph.addImport(edge.from, resolved, edge.file, edge.to, edge.line)
	}
	return graph
}

func typeScriptImportSpecifiers(source string) []scriptImport {
	imports := make([]scriptImport, 0)
	seen := make(map[string]struct{})
	for _, pattern := range typeScriptImportSpecifierPatterns {
		for _, match := range pattern.FindAllStringSubmatchIndex(source, -1) {
			specifier := source[match[2]:match[3]]
			line := support.LineNumberForOffset(source, match[0])
			key := specifier + ":" + strconv.Itoa(line)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			imports = append(imports, scriptImport{specifier: specifier, line: line})
		}
	}
	return imports
}

func typeScriptModuleKey(rel string) string {
	rel = strings.TrimPrefix(path.Clean(strings.ReplaceAll(rel, "\\", "/")), "./")
	for _, ext := range typeScriptModuleExtensions {
		if strings.HasSuffix(rel, ext) {
			return strings.TrimSuffix(rel, ext)
		}
	}
	return rel
}

// resolveTypeScriptImport resolves a relative import specifier to a known
// module key; external package imports return an empty string.
func resolveTypeScriptImport(graph *moduleGraph, fromModule string, specifier string) string {
	if !strings.HasPrefix(specifier, "./") && !strings.HasPrefix(specifier, "../") && specifier != "." && specifier != ".." {
		return ""
	}
	joined := path.Clean(path.Join(path.Dir(fromModule), specifier))
	joined = typeScriptModuleKey(joined)
	for _, candidate := range []string{joined, joined + "/index"} {
		if _, ok := graph.modules[candidate]; ok {
			return candidate
		}
	}
	return ""
}
