package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	cppToolchainFunctionDefinitionPattern = regexp.MustCompile(`(?ms)(?:^|[;{}\n])\s*((?:static\s+)?(?:inline\s+)?(?:constexpr\s+)?(?:[\w:<>,~*&\s]+\s+)?)([A-Za-z_]\w*(?:::[A-Za-z_]\w*)*)\s*\([^;{}]*\)\s*(?:const\s*)?(?:noexcept\s*)?(?:->\s*[\w:<>,~*&\s]+)?\s*\{`)
	cppToolchainSymbolTokenPattern        = regexp.MustCompile(`[?_$A-Za-z][A-Za-z0-9_:$?@.]*`)
	cppToolchainSourcePathTokenPattern    = regexp.MustCompile(`[A-Za-z0-9_./\\+\-]+\.(?:C|cc|cp|cpp|cxx|c\+\+|ixx|cppm|cxxm|ccm|c\+\+m|mpp|mxx)`)
	cppToolchainObjectPathTokenPattern    = regexp.MustCompile(`[A-Za-z0-9_./\\+\-]+\.(?:o|obj)`)
)

func cppToolchainDeadCodeFindings(env support.Context, target core.TargetConfig) []core.Finding {
	cfg := env.Config.Checks.QualityRules.DeadCode
	if !goToolchainDeadCodeEnabled(cfg) || !isExplicitCPPTarget(target.Language) {
		return nil
	}
	level := goToolchainDeadCodeLevel(cfg)
	candidates := cppToolchainDeadCodeCandidates(env, target, cfg)
	evidence := cppToolchainReadLinkerEvidence(env, target, cfg.CPP.Reports)
	findings := cppToolchainLinkerEvidenceFindings(env, level, evidence, candidates)
	if cfg.CPP.Graph != nil && !*cfg.CPP.Graph {
		return findings
	}
	entrypoints := cppToolchainEntrypoints(env, target, cfg.CPP.Entrypoints)
	if len(entrypoints) == 0 {
		if len(findings) > 0 {
			return findings
		}
		return []core.Finding{goToolchainDeadCodeDiagnostic(env, level, "quality_rules.dead_code.cpp.entrypoints must name at least one C++ entrypoint file or glob")}
	}
	dependencyGraph := support.BuildCPPDependencyGraph(env, target)
	if dependencyGraph == nil {
		return findings
	}
	reachable := cppToolchainReachableFiles(dependencyGraph.Graph, entrypoints)
	for _, candidate := range candidates {
		if reachable[candidate.rel] {
			continue
		}
		if cppToolchainCandidateHasLinkerDiscardEvidence(evidence, candidate) {
			continue
		}
		if cppToolchainCandidateHasLiveSymbolEvidence(evidence, candidate) {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     goToolchainDeadCodeRuleID,
			Level:      level,
			Path:       candidate.rel,
			Line:       1,
			Column:     1,
			Confidence: candidate.confidence,
			Message: fmt.Sprintf(
				"C++ %s %q is outside the include/module graph for configured entrypoints (%s)",
				candidate.kind, candidate.rel, strings.Join(entrypoints, ", "),
			),
			Metadata: map[string]string{
				"language": "cpp",
				"kind":     candidate.kind,
				"evidence": "include-module-graph",
			},
		}))
	}
	return findings
}

type cppToolchainDeadCodeCandidate struct {
	rel        string
	kind       string
	confidence string
	symbols    []cppToolchainSymbolCandidate
}

type cppToolchainSymbolCandidate struct {
	name   string
	line   int
	column int
	local  bool
}

func cppToolchainEntrypoints(env support.Context, target core.TargetConfig, configured []string) []string {
	patterns := append([]string(nil), configured...)
	if len(patterns) == 0 {
		patterns = append(patterns, target.Entrypoints...)
	}
	if len(patterns) == 0 {
		return nil
	}
	files := cppToolchainFileList(env, target)
	cppFiles := make([]string, 0, len(files))
	for _, rel := range files {
		if support.IsCPPPath(rel, true) {
			cppFiles = append(cppFiles, rel)
		}
	}
	seen := map[string]struct{}{}
	entrypoints := make([]string, 0)
	patterns = cppToolchainEntrypointPatterns(patterns)
	for _, rel := range cppFiles {
		if !cppToolchainMatchesEntrypointPattern(rel, patterns) {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		entrypoints = append(entrypoints, rel)
	}
	sort.Strings(entrypoints)
	return entrypoints
}

func cppToolchainEntrypointPatterns(configured []string) []string {
	patterns := make([]string, 0, len(configured))
	for _, pattern := range configured {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern != "" {
			patterns = append(patterns, pattern)
		}
	}
	return patterns
}

func cppToolchainMatchesEntrypointPattern(rel string, patterns []string) bool {
	for _, pattern := range patterns {
		if rel == pattern || support.PathMatchesPattern(pattern, rel) {
			return true
		}
	}
	return false
}

func cppToolchainReachableFiles(graph support.DependencyGraph, entrypoints []string) map[string]bool {
	reachable := make(map[string]bool, len(graph.Nodes))
	queue := append([]string(nil), entrypoints...)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if reachable[current] {
			continue
		}
		node, ok := graph.Nodes[current]
		if !ok {
			continue
		}
		reachable[current] = true
		for _, edge := range node.Edges {
			if !reachable[edge.To] {
				queue = append(queue, edge.To)
			}
		}
	}
	return reachable
}

func cppToolchainDeadCodeCandidates(env support.Context, target core.TargetConfig, cfg core.QualityDeadCodeConfig) []cppToolchainDeadCodeCandidate {
	includeTests := cfg.IncludeTests != nil && *cfg.IncludeTests
	candidates := make([]cppToolchainDeadCodeCandidate, 0)
	env.VisitTargetFiles(target, func(rel string) bool {
		if !support.IsCPPPath(rel, true) || !cppToolchainReportablePath(rel) {
			return false
		}
		if !includeTests && cppToolchainTestPath(rel) {
			return false
		}
		if cppToolchainVendorPath(rel) {
			return false
		}
		for _, pattern := range cfg.CPP.IgnorePaths {
			if support.PathMatchesPattern(pattern, rel) {
				return false
			}
		}
		return true
	}, func(rel string, data []byte) {
		source := string(data)
		if cppToolchainGeneratedFile(source) || cppToolchainDynamicSurface(source) {
			return
		}
		kind := "translation unit"
		confidence := core.ConfidenceLow
		if cppToolchainModulePath(rel) || cppToolchainDeclaresModule(source) {
			kind = "module"
			confidence = core.ConfidenceMedium
		}
		candidates = append(candidates, cppToolchainDeadCodeCandidate{
			rel:        filepath.ToSlash(rel),
			kind:       kind,
			confidence: confidence,
			symbols:    cppToolchainSymbolCandidates(source),
		})
	})
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].rel < candidates[j].rel })
	return candidates
}

func cppToolchainFileList(env support.Context, target core.TargetConfig) []string {
	if env.ListTargetFiles != nil {
		files, err := env.ListTargetFiles(target)
		if err == nil {
			for idx := range files {
				files[idx] = filepath.ToSlash(files[idx])
			}
			sort.Strings(files)
			return files
		}
	}
	files := make([]string, 0)
	env.VisitTargetFiles(target, func(string) bool { return true }, func(rel string, _ []byte) {
		files = append(files, filepath.ToSlash(rel))
	})
	sort.Strings(files)
	return files
}

func cppToolchainReportablePath(rel string) bool {
	rawExt := filepath.Ext(rel)
	if rawExt == ".C" {
		return true
	}
	switch strings.ToLower(rawExt) {
	case ".cc", ".cp", ".cpp", ".cxx", ".c++",
		".ixx", ".cppm", ".cxxm", ".ccm", ".c++m", ".mpp", ".mxx":
		return true
	default:
		return false
	}
}

func cppToolchainModulePath(rel string) bool {
	rawExt := filepath.Ext(rel)
	if rawExt == ".C" {
		return false
	}
	switch strings.ToLower(rawExt) {
	case ".ixx", ".cppm", ".cxxm", ".ccm", ".c++m", ".mpp", ".mxx":
		return true
	default:
		return false
	}
}

func cppToolchainTestPath(rel string) bool {
	normalized := strings.ToLower(filepath.ToSlash(rel))
	base := strings.TrimSuffix(filepath.Base(normalized), filepath.Ext(normalized))
	if strings.Contains(normalized, "/test/") ||
		strings.Contains(normalized, "/tests/") ||
		strings.Contains(normalized, "/testing/") ||
		strings.Contains(normalized, "/fixture/") ||
		strings.Contains(normalized, "/fixtures/") ||
		strings.Contains(normalized, "/testdata/") {
		return true
	}
	return strings.HasSuffix(base, "_test") ||
		strings.HasSuffix(base, "-test") ||
		strings.HasSuffix(base, "_tests") ||
		strings.HasSuffix(base, "-tests")
}

func cppToolchainVendorPath(rel string) bool {
	normalized := strings.ToLower(filepath.ToSlash(rel))
	return strings.HasPrefix(normalized, "vendor/") ||
		strings.Contains(normalized, "/vendor/") ||
		strings.HasPrefix(normalized, "third_party/") ||
		strings.Contains(normalized, "/third_party/") ||
		strings.HasPrefix(normalized, "third-party/") ||
		strings.Contains(normalized, "/third-party/") ||
		strings.HasPrefix(normalized, "external/") ||
		strings.Contains(normalized, "/external/") ||
		strings.HasPrefix(normalized, "generated/") ||
		strings.Contains(normalized, "/generated/")
}

func cppToolchainGeneratedFile(source string) bool {
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "Code generated") && strings.Contains(trimmed, "DO NOT EDIT") {
			return true
		}
		if strings.Contains(strings.ToLower(trimmed), "@generated") {
			return true
		}
		if !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "/*") && !strings.HasPrefix(trimmed, "*") {
			return false
		}
	}
	return false
}

func cppToolchainDynamicSurface(source string) bool {
	masked := support.ParseCLike(source, support.CLikeCPP).Masked
	indicators := []string{
		`extern "C"`,
		"__declspec(dllexport)",
		"__declspec(dllimport)",
		"__attribute__((visibility",
		"__attribute__((constructor",
		"JNIEXPORT",
		"PYBIND11_MODULE",
		"BOOST_PYTHON_MODULE",
		"NODE_MODULE",
		"NAPI_MODULE",
		"Q_OBJECT",
		"Q_PLUGIN_METADATA",
		"G_DEFINE_TYPE",
		"G_DECLARE_",
		"REGISTER_",
		"_REGISTER",
		"REGISTER(",
		"Register",
		"register",
		"callback",
		"Callback",
		"plugin",
		"Plugin",
		"template <",
		"virtual ",
		" override",
	}
	for _, indicator := range indicators {
		if strings.Contains(masked, indicator) {
			return true
		}
	}
	return false
}

func cppToolchainDeclaresModule(source string) bool {
	parsed := support.ParseCLike(source, support.CLikeCPP)
	return support.CPPModuleDeclarationPattern.FindStringSubmatch(parsed.Masked) != nil
}

type cppToolchainLinkerEvidence struct {
	reports          []string
	liveSymbols      map[string]struct{}
	discardedSymbols map[string]struct{}
	discardedFiles   map[string]struct{}
	discardedObjects map[string]struct{}
}

func cppToolchainReadLinkerEvidence(env support.Context, target core.TargetConfig, configured []string) cppToolchainLinkerEvidence {
	evidence := cppToolchainNewLinkerEvidence()
	for _, rel := range cppToolchainReportPaths(env, target, configured) {
		data, err := cppToolchainReadTargetFile(env, target, rel)
		if err != nil {
			continue
		}
		evidence.reports = append(evidence.reports, rel)
		cppToolchainAddLinkerEvidence(&evidence, rel, string(data))
	}
	sort.Strings(evidence.reports)
	return evidence
}

func cppToolchainNewLinkerEvidence() cppToolchainLinkerEvidence {
	return cppToolchainLinkerEvidence{
		liveSymbols:      map[string]struct{}{},
		discardedSymbols: map[string]struct{}{},
		discardedFiles:   map[string]struct{}{},
		discardedObjects: map[string]struct{}{},
	}
}

func cppToolchainReportPaths(env support.Context, target core.TargetConfig, configured []string) []string {
	seen := map[string]struct{}{}
	add := func(rel string) {
		rel = filepath.ToSlash(strings.TrimSpace(rel))
		if rel == "" {
			return
		}
		if _, ok := seen[rel]; ok {
			return
		}
		seen[rel] = struct{}{}
	}
	files := cppToolchainFileList(env, target)
	for _, pattern := range configured {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if strings.ContainsAny(pattern, "*?[") {
			for _, rel := range files {
				if support.PathMatchesPattern(pattern, rel) {
					add(rel)
				}
			}
			continue
		}
		add(pattern)
	}
	reports := make([]string, 0, len(seen))
	for rel := range seen {
		reports = append(reports, rel)
	}
	sort.Strings(reports)
	return reports
}

func cppToolchainReadTargetFile(env support.Context, target core.TargetConfig, rel string) ([]byte, error) {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if env.ReadTargetFile != nil {
		if data, err := env.ReadTargetFile(target, rel); err == nil {
			return data, nil
		}
	}
	return os.ReadFile(filepath.Join(target.Path, filepath.FromSlash(rel)))
}

func cppToolchainAddLinkerEvidence(evidence *cppToolchainLinkerEvidence, _ string, data string) {
	for _, rawLine := range strings.Split(data, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		discard := cppToolchainDiscardLine(lower)
		if discard {
			for _, rel := range cppToolchainSourcePathTokenPattern.FindAllString(line, -1) {
				evidence.discardedFiles[cppToolchainNormalizeArtifactPath(rel)] = struct{}{}
			}
			if cppToolchainDiscardObjectLine(lower) {
				for _, obj := range cppToolchainObjectPathTokenPattern.FindAllString(line, -1) {
					evidence.discardedObjects[cppToolchainNormalizeObjectStem(obj)] = struct{}{}
				}
			}
		}
		for _, symbol := range cppToolchainSymbolsFromReportLine(line) {
			if !cppToolchainSafeReportSymbol(symbol) {
				continue
			}
			if discard {
				if cppToolchainWeakDiscardLine(lower) {
					continue
				}
				evidence.discardedSymbols[symbol] = struct{}{}
				continue
			}
			if cppToolchainLiveSymbolLine(line) {
				evidence.liveSymbols[symbol] = struct{}{}
			}
		}
	}
}

func cppToolchainDiscardLine(lower string) bool {
	return strings.Contains(lower, "removing unused section") ||
		strings.Contains(lower, "discarded") ||
		strings.Contains(lower, "discarding") ||
		strings.Contains(lower, "dead strip") ||
		strings.Contains(lower, "dead_strip") ||
		strings.Contains(lower, "<<dead>>") ||
		strings.Contains(lower, "dead stripped")
}

func cppToolchainDiscardObjectLine(lower string) bool {
	return strings.Contains(lower, "discarded object") ||
		strings.Contains(lower, "discarding object") ||
		strings.Contains(lower, "discarded input file") ||
		strings.Contains(lower, "discarding input file") ||
		strings.Contains(lower, "discarding file")
}

func cppToolchainWeakDiscardLine(lower string) bool {
	return strings.Contains(lower, " weak ") ||
		strings.Contains(lower, "\tweak\t") ||
		strings.Contains(lower, " weak_def")
}

func cppToolchainLiveSymbolLine(line string) bool {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return false
	}
	lower := strings.ToLower(line)
	if strings.Contains(lower, "(__text,__text)") ||
		strings.Contains(lower, "(__text,__stubs)") ||
		strings.Contains(lower, "(__text,__cstring)") ||
		strings.Contains(lower, "(__text,__const)") ||
		strings.Contains(lower, "external") && strings.Contains(lower, "|") ||
		strings.Contains(lower, "static") && strings.Contains(lower, "|") {
		return true
	}
	for idx, field := range fields {
		if len(field) != 1 || idx == len(fields)-1 {
			continue
		}
		switch field[0] {
		case 'T', 't', 'W', 'w', 'V', 'v', 'D', 'd', 'B', 'b', 'R', 'r', 'S', 's':
			return true
		case 'U', 'u':
			return false
		}
	}
	return false
}

func cppToolchainSymbolsFromReportLine(line string) []string {
	tokens := cppToolchainSymbolTokenPattern.FindAllString(line, -1)
	symbols := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if cppToolchainReportNoiseToken(token) {
			continue
		}
		symbols = append(symbols, token)
	}
	return symbols
}

func cppToolchainReportNoiseToken(token string) bool {
	lower := strings.ToLower(strings.Trim(token, " \t:"))
	if len(lower) < 3 {
		return true
	}
	switch lower {
	case "removing", "unused", "section", "file", "discarded", "discarding", "dead", "strip",
		"text", "data", "bss", "linker", "archive", "object", "build", "debug", "weak":
		return true
	default:
		return false
	}
}

func cppToolchainSafeReportSymbol(symbol string) bool {
	lower := strings.ToLower(symbol)
	return !strings.Contains(lower, "vtable") &&
		!strings.Contains(lower, "typeinfo") &&
		!strings.Contains(lower, "virtual") &&
		!strings.Contains(lower, "thunk") &&
		!strings.Contains(lower, "__cxx_global_var_init") &&
		!strings.Contains(lower, "_global__sub_i") &&
		!strings.Contains(lower, "guard variable")
}

func cppToolchainNormalizeArtifactPath(rel string) string {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	for strings.HasPrefix(rel, "./") {
		rel = strings.TrimPrefix(rel, "./")
	}
	return rel
}

func cppToolchainNormalizeObjectStem(rel string) string {
	rel = cppToolchainNormalizeArtifactPath(rel)
	base := filepath.Base(rel)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func cppToolchainLinkerEvidenceFindings(env support.Context, level string, evidence cppToolchainLinkerEvidence, candidates []cppToolchainDeadCodeCandidate) []core.Finding {
	if len(evidence.reports) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0)
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if cppToolchainCandidateFileDiscarded(evidence, candidate) {
			key := candidate.rel + ":file"
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:     goToolchainDeadCodeRuleID,
				Level:      level,
				Path:       candidate.rel,
				Line:       1,
				Column:     1,
				Confidence: core.ConfidenceHigh,
				Message: fmt.Sprintf(
					"C++ translation unit %q is reported as discarded by configured linker artifacts (%s)",
					candidate.rel, strings.Join(evidence.reports, ", "),
				),
				Metadata: map[string]string{
					"language": "cpp",
					"kind":     candidate.kind,
					"evidence": "linker-discard",
				},
			}))
			continue
		}
		for _, symbol := range candidate.symbols {
			if !symbol.local || !cppToolchainSymbolDiscarded(evidence, symbol.name) {
				continue
			}
			key := candidate.rel + ":" + symbol.name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:     goToolchainDeadCodeRuleID,
				Level:      level,
				Path:       candidate.rel,
				Line:       symbol.line,
				Column:     symbol.column,
				Confidence: core.ConfidenceHigh,
				Message: fmt.Sprintf(
					"C++ local symbol %q is reported as discarded by configured linker artifacts (%s)",
					symbol.name, strings.Join(evidence.reports, ", "),
				),
				Metadata: map[string]string{
					"language": "cpp",
					"kind":     "symbol",
					"symbol":   symbol.name,
					"evidence": "linker-discard",
				},
			}))
		}
	}
	return findings
}

func cppToolchainCandidateHasLinkerDiscardEvidence(evidence cppToolchainLinkerEvidence, candidate cppToolchainDeadCodeCandidate) bool {
	if len(evidence.reports) == 0 {
		return false
	}
	if cppToolchainCandidateFileDiscarded(evidence, candidate) {
		return true
	}
	for _, symbol := range candidate.symbols {
		if symbol.local && cppToolchainSymbolDiscarded(evidence, symbol.name) {
			return true
		}
	}
	return false
}

func cppToolchainCandidateFileDiscarded(evidence cppToolchainLinkerEvidence, candidate cppToolchainDeadCodeCandidate) bool {
	normalized := cppToolchainNormalizeArtifactPath(candidate.rel)
	if _, ok := evidence.discardedFiles[normalized]; ok {
		return true
	}
	base := filepath.Base(normalized)
	if _, ok := evidence.discardedFiles[base]; ok {
		return true
	}
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if _, ok := evidence.discardedObjects[stem]; ok {
		return true
	}
	return false
}

func cppToolchainCandidateHasLiveSymbolEvidence(evidence cppToolchainLinkerEvidence, candidate cppToolchainDeadCodeCandidate) bool {
	if len(evidence.liveSymbols) == 0 {
		return false
	}
	for _, symbol := range candidate.symbols {
		if cppToolchainSymbolLive(evidence, symbol.name) {
			return true
		}
	}
	return false
}

func cppToolchainSymbolDiscarded(evidence cppToolchainLinkerEvidence, name string) bool {
	for symbol := range evidence.discardedSymbols {
		if cppToolchainReportSymbolMatchesName(symbol, name) {
			return true
		}
	}
	return false
}

func cppToolchainSymbolLive(evidence cppToolchainLinkerEvidence, name string) bool {
	for symbol := range evidence.liveSymbols {
		if cppToolchainReportSymbolMatchesName(symbol, name) {
			return true
		}
	}
	return false
}

func cppToolchainReportSymbolMatchesName(symbol string, name string) bool {
	if symbol == name || strings.HasSuffix(symbol, "::"+name) {
		return true
	}
	if strings.Contains(symbol, name) {
		return true
	}
	lowerSymbol := strings.ToLower(symbol)
	lowerName := strings.ToLower(name)
	return lowerSymbol == lowerName ||
		strings.HasSuffix(lowerSymbol, "::"+lowerName) ||
		strings.Contains(lowerSymbol, lowerName)
}

func cppToolchainSymbolCandidates(source string) []cppToolchainSymbolCandidate {
	masked := support.ParseCLike(source, support.CLikeCPP).Masked
	matches := cppToolchainFunctionDefinitionPattern.FindAllStringSubmatchIndex(masked, -1)
	candidates := make([]cppToolchainSymbolCandidate, 0, len(matches))
	for _, match := range matches {
		if len(match) < 6 || match[4] < 0 || match[5] < 0 {
			continue
		}
		prefix := strings.TrimSpace(masked[match[2]:match[3]])
		name := masked[match[4]:match[5]]
		if cppToolchainControlKeyword(name) {
			continue
		}
		unqualified := name
		if idx := strings.LastIndex(unqualified, "::"); idx >= 0 {
			unqualified = unqualified[idx+2:]
		}
		if unqualified == "" || strings.HasPrefix(unqualified, "~") {
			continue
		}
		line, column := cppToolchainLineColumn(masked, match[4])
		candidates = append(candidates, cppToolchainSymbolCandidate{
			name:   unqualified,
			line:   line,
			column: column,
			local:  cppToolchainLocalFunctionPrefix(prefix),
		})
	}
	return candidates
}

func cppToolchainControlKeyword(name string) bool {
	switch name {
	case "if", "for", "while", "switch", "catch", "return", "sizeof":
		return true
	default:
		return false
	}
}

func cppToolchainLocalFunctionPrefix(prefix string) bool {
	fields := strings.Fields(prefix)
	for _, field := range fields {
		if field == "static" {
			return true
		}
	}
	return false
}

func cppToolchainLineColumn(source string, offset int) (int, int) {
	line := 1
	column := 1
	for idx, r := range source {
		if idx >= offset {
			break
		}
		if r == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return line, column
}
