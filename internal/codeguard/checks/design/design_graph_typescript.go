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
	resolver := newTypeScriptImportResolver(graph)
	env.VisitTargetFiles(target, isTypeScriptResolverMetadataFile, func(rel string, data []byte) {
		resolver.indexMetadata(rel, data)
	})
	resolver.finalizeConfigs()
	for _, edge := range pending {
		resolved := resolveTypeScriptImport(resolver, edge.from, edge.to)
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

// resolveTypeScriptImport resolves a script import specifier to a known local
// module key using relative imports, tsconfig aliases/baseUrl, and local
// workspace package manifests. External package imports return an empty string.
func resolveTypeScriptImport(resolver *typeScriptImportResolver, fromModule string, specifier string) string {
	if resolver == nil {
		return ""
	}
	return resolver.resolve(fromModule, specifier)
}
