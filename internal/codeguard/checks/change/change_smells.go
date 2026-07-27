package change

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	wordTokenPattern              = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*|\d+|==|!=|<=|>=|&&|\|\||[{}()[\].,:;+*/%<>=!?-]`)
	goInterfaceDeclPattern        = regexp.MustCompile(`^\s*type\s+([A-Za-z_][A-Za-z0-9_]*)\s+interface\b`)
	scriptInterfaceDeclPattern    = regexp.MustCompile(`^\s*(?:export\s+)?interface\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	abstractClassDeclPattern      = regexp.MustCompile(`^\s*(?:export\s+)?abstract\s+class\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	cppAbstractClassDeclPattern   = regexp.MustCompile(`^\s*class\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	functionDeclPattern           = regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?(?:func|function|def)\s+([A-Za-z_][A-Za-z0-9_]*)\b|^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(?:async\s*)?\([^)]*\)\s*=>`)
	cppFunctionDeclPattern        = regexp.MustCompile(`^\s*(?:[A-Za-z_][A-Za-z0-9_:<>,*&\s]+)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\([^;{}]*\)\s*(?:const\s*)?\{`)
	complexityTokenPattern        = regexp.MustCompile(`\b(if|else\s+if|elif|for|while|switch|case|catch|except|guard|when)\b|&&|\|\||\?`)
	dependencyLinePattern         = regexp.MustCompile(`^\s*(?:import\b|from\s+\S+\s+import\b|#include\b|using\s+namespace\b|const\s+\w+\s*=\s*require\()`)
	pythonFunctionDeclPattern     = regexp.MustCompile(`^def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	publicDeclPatternsByExtension = map[string][]*regexp.Regexp{
		".go":  {regexp.MustCompile(`^\s*(?:type|func|const|var)\s+[A-Z][A-Za-z0-9_]*\b`)},
		".py":  {regexp.MustCompile(`^\s*class\s+[A-Z][A-Za-z0-9_]*\b`), regexp.MustCompile(`^\s*def\s+[A-Za-z][A-Za-z0-9_]*\b`)},
		".ts":  {regexp.MustCompile(`^\s*export\s+(?:interface|type|class|function|const|let|var)\s+[A-Za-z_][A-Za-z0-9_]*\b`)},
		".tsx": {regexp.MustCompile(`^\s*export\s+(?:interface|type|class|function|const|let|var)\s+[A-Za-z_][A-Za-z0-9_]*\b`)},
		".js":  {regexp.MustCompile(`^\s*export\s+(?:class|function|const|let|var)\s+[A-Za-z_][A-Za-z0-9_]*\b`)},
		".jsx": {regexp.MustCompile(`^\s*export\s+(?:class|function|const|let|var)\s+[A-Za-z_][A-Za-z0-9_]*\b`)},
		".h":   {regexp.MustCompile(`^\s*(?:class|struct|enum)\s+[A-Za-z_][A-Za-z0-9_]*\b`)},
		".hpp": {regexp.MustCompile(`^\s*(?:class|struct|enum)\s+[A-Za-z_][A-Za-z0-9_]*\b`)},
		".hh":  {regexp.MustCompile(`^\s*(?:class|struct|enum)\s+[A-Za-z_][A-Za-z0-9_]*\b`)},
	}
)

type changedFileContent struct {
	path    string
	status  core.ChangedFileStatus
	target  core.TargetConfig
	head    []byte
	base    []byte
	hasBase bool
	ranges  core.ChangedLineRanges
}

type abstractionDecl struct {
	name string
	path string
	line int
}

type helperFunction struct {
	name       string
	path       string
	line       int
	body       string
	normalized string
	tokens     int
	changed    bool
}

type fileMaintainabilityMetrics struct {
	complexity int
	nesting    int
	deps       int
	public     int
}

func changeSmellFindings(env support.Context, ev evidence) []core.Finding {
	if env.Mode != core.ScanModeDiff || len(ev.files) == 0 || env.ReadTargetFile == nil {
		return nil
	}
	files := changedSourceContents(env, ev)
	if len(files) == 0 {
		return nil
	}

	rules := env.Config.Checks.ChangeRules
	findings := make([]core.Finding, 0, 4)
	if enabled(rules.DetectOneUseAbstraction) {
		findings = append(findings, oneUseAbstractionFindings(env, files)...)
	}
	if enabled(rules.DetectDuplicateHelper) {
		findings = append(findings, duplicateHelperFindings(env, files)...)
	}
	if enabled(rules.DetectComplexityIncreased) {
		findings = append(findings, complexityIncreasedFindings(env, files)...)
	}
	if enabled(rules.DetectCleanupRegression) {
		findings = append(findings, cleanupRegressionFindings(env, files)...)
	}
	return findings
}

func changedSourceContents(env support.Context, ev evidence) []changedFileContent {
	scope := map[string]core.ChangedLineRanges{}
	if env.DiffScope != nil {
		scope = env.DiffScope()
	}
	out := make([]changedFileContent, 0, len(ev.files))
	for _, file := range ev.files {
		if file.status == core.ChangedFileDeleted || !isProductionFile(file.path) || !isSourceFile(file.path) {
			continue
		}
		if isAdapterPath(file.path) {
			continue
		}
		for _, target := range env.Config.Targets {
			head, err := env.ReadTargetFile(target, file.path)
			if err != nil {
				continue
			}
			base, baseErr := readBaseFile(env, target, file.path)
			out = append(out, changedFileContent{
				path:    file.path,
				status:  file.status,
				target:  target,
				head:    head,
				base:    base,
				hasBase: baseErr == nil,
				ranges:  scope[file.path],
			})
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func readBaseFile(env support.Context, target core.TargetConfig, rel string) ([]byte, error) {
	if env.ReadBaseFile == nil {
		return nil, fmt.Errorf("base file callback is not configured")
	}
	return env.ReadBaseFile(target, rel)
}

func oneUseAbstractionFindings(env support.Context, files []changedFileContent) []core.Finding {
	decls := make([]abstractionDecl, 0, len(files)*2)
	for _, file := range files {
		decls = append(decls, newAbstractionDecls(file)...)
	}
	if len(decls) == 0 {
		return nil
	}

	repoText := productionSourceText(env)
	findings := make([]core.Finding, 0, len(decls))
	for _, decl := range decls {
		uses := countWord(repoText, decl.name)
		if uses > 2 {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     "change.one-use-abstraction",
			Level:      "warn",
			Path:       decl.path,
			Line:       decl.line,
			Confidence: "medium",
			Message:    fmt.Sprintf("new abstraction %s has only %d repository reference(s); keep the boundary only if it carries real policy or has another concrete consumer", decl.name, uses),
			Metadata: map[string]string{
				"abstraction": decl.name,
				"references":  strconv.Itoa(uses),
			},
		}))
	}
	return findings
}

func newAbstractionDecls(file changedFileContent) []abstractionDecl {
	lines := strings.Split(string(file.head), "\n")
	out := make([]abstractionDecl, 0)
	for idx, raw := range lines {
		lineNo := idx + 1
		if !lineIsChanged(file.ranges, lineNo) {
			continue
		}
		line := strings.TrimSpace(maskLineComments(raw))
		if line == "" {
			continue
		}
		name, ok := abstractionNameForLine(file.path, line)
		if !ok || abstractionDeclaredInBase(file, name) {
			continue
		}
		out = append(out, abstractionDecl{name: name, path: file.path, line: lineNo})
	}
	return out
}

func abstractionNameForLine(rel string, line string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(rel))
	switch ext {
	case ".go":
		if m := goInterfaceDeclPattern.FindStringSubmatch(line); len(m) == 2 {
			return m[1], true
		}
	case ".ts", ".tsx", ".js", ".jsx":
		if m := scriptInterfaceDeclPattern.FindStringSubmatch(line); len(m) == 2 {
			return m[1], true
		}
		if m := abstractClassDeclPattern.FindStringSubmatch(line); len(m) == 2 {
			return m[1], true
		}
	case ".h", ".hpp", ".hh":
		if m := cppAbstractClassDeclPattern.FindStringSubmatch(line); len(m) == 2 && strings.Contains(line, "virtual") {
			return m[1], true
		}
	}
	return "", false
}

func abstractionDeclaredInBase(file changedFileContent, name string) bool {
	if !file.hasBase {
		return false
	}
	for _, line := range strings.Split(string(file.base), "\n") {
		if baseName, ok := abstractionNameForLine(file.path, strings.TrimSpace(maskLineComments(line))); ok && baseName == name {
			return true
		}
	}
	return false
}

func duplicateHelperFindings(env support.Context, files []changedFileContent) []core.Finding {
	all := allProductionFunctions(env)
	if len(all) < 2 {
		return nil
	}
	changed := changedHelperFunctions(files)
	findings := make([]core.Finding, 0)
	seen := map[string]struct{}{}
	for _, candidate := range changed {
		if candidate.tokens < 18 || !isHelperLikeName(candidate.name) {
			continue
		}
		for _, other := range all {
			if sameFunctionLocation(candidate, other) || other.normalized != candidate.normalized {
				continue
			}
			key := candidate.path + ":" + candidate.name
			if _, exists := seen[key]; exists {
				break
			}
			seen[key] = struct{}{}
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:     "change.duplicate-helper",
				Level:      "warn",
				Path:       candidate.path,
				Line:       candidate.line,
				Confidence: "high",
				Message:    fmt.Sprintf("changed helper %s duplicates %s in %s:%d; reuse one shared implementation instead of adding a parallel helper", candidate.name, other.name, other.path, other.line),
				Metadata: map[string]string{
					"helper":         candidate.name,
					"duplicate_of":   other.name,
					"duplicate_path": other.path,
				},
			}))
			break
		}
	}
	return findings
}

func changedHelperFunctions(files []changedFileContent) []helperFunction {
	out := make([]helperFunction, 0)
	for _, file := range files {
		for _, fn := range extractFunctions(file.path, string(file.head)) {
			if !functionIntersectsChangedLines(fn, file.ranges) {
				continue
			}
			fn.changed = true
			out = append(out, fn)
		}
	}
	sortFunctions(out)
	return out
}

func allProductionFunctions(env support.Context) []helperFunction {
	out := make([]helperFunction, 0)
	for _, target := range env.Config.Targets {
		if env.VisitTargetFiles != nil {
			env.VisitTargetFiles(target, func(rel string) bool {
				return isProductionFile(rel) && isSourceFile(rel) && !isGeneratedPath(rel) && !isAdapterPath(rel)
			}, func(rel string, data []byte) {
				out = append(out, extractFunctions(rel, string(data))...)
			})
			continue
		}
		if env.ListTargetFiles == nil || env.ReadTargetFile == nil {
			continue
		}
		files, err := env.ListTargetFiles(target)
		if err != nil {
			continue
		}
		sort.Strings(files)
		for _, rel := range files {
			if !isProductionFile(rel) || !isSourceFile(rel) || isGeneratedPath(rel) || isAdapterPath(rel) {
				continue
			}
			data, err := env.ReadTargetFile(target, rel)
			if err == nil {
				out = append(out, extractFunctions(rel, string(data))...)
			}
		}
	}
	sortFunctions(out)
	return out
}

func extractFunctions(rel string, source string) []helperFunction {
	if strings.HasSuffix(strings.ToLower(rel), ".py") {
		return extractPythonFunctions(rel, source)
	}
	return extractBraceFunctions(rel, source)
}

func extractBraceFunctions(rel string, source string) []helperFunction {
	lines := strings.Split(source, "\n")
	out := make([]helperFunction, 0)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		name, ok := braceFunctionName(rel, line)
		if !ok {
			continue
		}
		bodyLines, end, ok := collectBraceBody(lines, i)
		if !ok {
			continue
		}
		normalized, tokens := normalizedTokenText(normalizedBraceBody(bodyLines))
		if tokens > 0 {
			out = append(out, helperFunction{name: name, path: rel, line: i + 1, body: strings.Join(bodyLines, "\n"), normalized: normalized, tokens: tokens})
		}
		i = end
	}
	return out
}

func braceFunctionName(rel string, line string) (string, bool) {
	line = strings.TrimSpace(maskLineComments(line))
	ext := strings.ToLower(filepath.Ext(rel))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		if m := functionDeclPattern.FindStringSubmatch(line); len(m) == 3 {
			if m[1] != "" {
				return m[1], true
			}
			return m[2], true
		}
	case ".c", ".cc", ".cpp", ".cxx", ".h", ".hpp", ".hh":
		if m := cppFunctionDeclPattern.FindStringSubmatch(line); len(m) == 2 {
			return m[1], true
		}
	}
	return "", false
}

func collectBraceBody(lines []string, start int) ([]string, int, bool) {
	depth := 0
	seenOpen := false
	body := make([]string, 0)
	for i := start; i < len(lines); i++ {
		line := lines[i]
		for _, r := range line {
			switch r {
			case '{':
				depth++
				seenOpen = true
			case '}':
				if depth > 0 {
					depth--
				}
			}
		}
		body = append(body, line)
		if seenOpen && depth == 0 {
			return body, i, true
		}
	}
	return nil, start, false
}

func normalizedBraceBody(lines []string) string {
	joined := strings.Join(lines, "\n")
	if idx := strings.Index(joined, "{"); idx >= 0 {
		joined = joined[idx+1:]
	}
	if idx := strings.LastIndex(joined, "}"); idx >= 0 {
		joined = joined[:idx]
	}
	return joined
}

func extractPythonFunctions(rel string, source string) []helperFunction {
	lines := strings.Split(source, "\n")
	out := make([]helperFunction, 0)
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(maskLineComments(lines[i]))
		m := pythonFunctionDeclPattern.FindStringSubmatch(line)
		if len(m) != 2 {
			continue
		}
		indent := leadingSpaces(lines[i])
		end := i + 1
		for end < len(lines) {
			trimmed := strings.TrimSpace(lines[end])
			if trimmed != "" && !isCommentOnly(trimmed) && leadingSpaces(lines[end]) <= indent {
				break
			}
			end++
		}
		body := strings.Join(lines[i:end], "\n")
		normalized, tokens := normalizedTokenText(strings.Join(lines[i+1:end], "\n"))
		if tokens > 0 {
			out = append(out, helperFunction{name: m[1], path: rel, line: i + 1, body: body, normalized: normalized, tokens: tokens})
		}
		i = end - 1
	}
	return out
}

func complexityIncreasedFindings(env support.Context, files []changedFileContent) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, file := range files {
		if !file.hasBase {
			continue
		}
		base := maintainabilityMetrics(string(file.base))
		head := maintainabilityMetrics(string(file.head))
		if !complexityRegression(base, head) {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     "change.complexity-increased",
			Level:      "warn",
			Path:       file.path,
			Line:       firstChangedComplexityLine(file),
			Confidence: confidenceForComplexityRegression(base, head),
			Message: fmt.Sprintf(
				"changed file complexity increased from %d to %d and max nesting from %d to %d; verify the added branching is necessary and covered",
				base.complexity,
				head.complexity,
				base.nesting,
				head.nesting,
			),
			Metadata: map[string]string{
				"base_complexity": strconv.Itoa(base.complexity),
				"head_complexity": strconv.Itoa(head.complexity),
				"base_nesting":    strconv.Itoa(base.nesting),
				"head_nesting":    strconv.Itoa(head.nesting),
			},
		}))
	}
	return findings
}

func cleanupRegressionFindings(env support.Context, files []changedFileContent) []core.Finding {
	if !changeClaimsCleanup(env) {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, file := range files {
		if !file.hasBase {
			continue
		}
		base := maintainabilityMetrics(string(file.base))
		head := maintainabilityMetrics(string(file.head))
		reasons := cleanupRegressionReasons(base, head)
		if len(reasons) == 0 {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     "change.cleanup-regression",
			Level:      "warn",
			Path:       file.path,
			Line:       firstChangedComplexityLine(file),
			Confidence: confidenceForReasonCount(len(reasons)),
			Message:    "cleanup/refactor-labeled change worsens maintainability signals: " + strings.Join(reasons, "; "),
			Metadata: map[string]string{
				"base_complexity": strconv.Itoa(base.complexity),
				"head_complexity": strconv.Itoa(head.complexity),
				"base_deps":       strconv.Itoa(base.deps),
				"head_deps":       strconv.Itoa(head.deps),
				"base_public":     strconv.Itoa(base.public),
				"head_public":     strconv.Itoa(head.public),
			},
		}))
	}
	return findings
}

func maintainabilityMetrics(source string) fileMaintainabilityMetrics {
	lines := strings.Split(source, "\n")
	metrics := fileMaintainabilityMetrics{}
	braceDepth := 0
	for _, raw := range lines {
		line := strings.TrimSpace(maskLineComments(raw))
		if line == "" || isCommentOnly(line) {
			continue
		}
		metrics.complexity += len(complexityTokenPattern.FindAllString(line, -1))
		if dependencyLinePattern.MatchString(strings.ToLower(line)) {
			metrics.deps++
		}
		if isPublicDeclarationLine(line, "") {
			metrics.public++
		}
		if strings.Contains(line, "{") || strings.Contains(line, "}") {
			for _, r := range line {
				if r == '{' {
					braceDepth++
					if braceDepth > metrics.nesting {
						metrics.nesting = braceDepth
					}
				}
				if r == '}' && braceDepth > 0 {
					braceDepth--
				}
			}
			continue
		}
		if indent := leadingSpaces(raw) / 4; indent > metrics.nesting {
			metrics.nesting = indent
		}
	}
	return metrics
}

func cleanupRegressionReasons(base fileMaintainabilityMetrics, head fileMaintainabilityMetrics) []string {
	reasons := make([]string, 0, 4)
	if complexityRegression(base, head) {
		reasons = append(reasons, fmt.Sprintf("complexity %d -> %d", base.complexity, head.complexity))
	}
	if head.nesting > base.nesting {
		reasons = append(reasons, fmt.Sprintf("nesting %d -> %d", base.nesting, head.nesting))
	}
	if head.public > base.public {
		reasons = append(reasons, fmt.Sprintf("public declarations %d -> %d", base.public, head.public))
	}
	if head.deps > base.deps+1 {
		reasons = append(reasons, fmt.Sprintf("direct dependencies %d -> %d", base.deps, head.deps))
	}
	return reasons
}

func complexityRegression(base fileMaintainabilityMetrics, head fileMaintainabilityMetrics) bool {
	return head.complexity >= base.complexity+2 || head.nesting > base.nesting+1
}

func confidenceForComplexityRegression(base fileMaintainabilityMetrics, head fileMaintainabilityMetrics) string {
	if head.complexity >= base.complexity+4 || head.nesting > base.nesting+2 {
		return "high"
	}
	return "medium"
}

func firstChangedComplexityLine(file changedFileContent) int {
	lines := strings.Split(string(file.head), "\n")
	for idx, raw := range lines {
		lineNo := idx + 1
		if !lineIsChanged(file.ranges, lineNo) {
			continue
		}
		line := strings.ToLower(strings.TrimSpace(maskLineComments(raw)))
		if complexityTokenPattern.MatchString(line) {
			return lineNo
		}
	}
	return firstChangedLine(file.ranges)
}

func changeClaimsCleanup(env support.Context) bool {
	haystack := strings.ToLower(env.Config.Name + "\n" + env.BaseRef + "\n" + env.DiffText)
	for _, token := range []string{"cleanup", "clean-up", "refactor", "simplify", "tidy", "chore"} {
		if strings.Contains(haystack, token) {
			return true
		}
	}
	return false
}

func productionSourceText(env support.Context) string {
	var b strings.Builder
	for _, target := range env.Config.Targets {
		if env.VisitTargetFiles != nil {
			env.VisitTargetFiles(target, func(rel string) bool {
				return isProductionFile(rel) && isSourceFile(rel) && !isGeneratedPath(rel) && !isAdapterPath(rel)
			}, func(_ string, data []byte) {
				b.Write(data)
				b.WriteByte('\n')
			})
			continue
		}
		if env.ListTargetFiles == nil || env.ReadTargetFile == nil {
			continue
		}
		files, err := env.ListTargetFiles(target)
		if err != nil {
			continue
		}
		sort.Strings(files)
		for _, rel := range files {
			if !isProductionFile(rel) || !isSourceFile(rel) || isGeneratedPath(rel) || isAdapterPath(rel) {
				continue
			}
			if data, err := env.ReadTargetFile(target, rel); err == nil {
				b.Write(data)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func normalizedTokenText(source string) (string, int) {
	matches := wordTokenPattern.FindAllString(source, -1)
	if len(matches) == 0 {
		return "", 0
	}
	for i, token := range matches {
		if isNumberToken(token) {
			matches[i] = "num"
			continue
		}
		matches[i] = strings.ToLower(token)
	}
	return strings.Join(matches, " "), len(matches)
}

func isNumberToken(token string) bool {
	if token == "" {
		return false
	}
	for i := 0; i < len(token); i++ {
		if token[i] < '0' || token[i] > '9' {
			return false
		}
	}
	return true
}

func isHelperLikeName(name string) bool {
	lower := strings.ToLower(name)
	for _, token := range []string{"helper", "normalize", "canonical", "parse", "format", "convert", "build", "make", "map", "validate", "sanitize", "clean"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func sameFunctionLocation(left helperFunction, right helperFunction) bool {
	return left.path == right.path && left.line == right.line
}

func functionIntersectsChangedLines(fn helperFunction, ranges core.ChangedLineRanges) bool {
	end := fn.line + strings.Count(fn.body, "\n")
	if ranges.AllChanged || len(ranges.Ranges) == 0 {
		return true
	}
	for _, r := range ranges.Ranges {
		if r[0] <= end && r[1] >= fn.line {
			return true
		}
	}
	return false
}

func lineIsChanged(ranges core.ChangedLineRanges, line int) bool {
	if ranges.AllChanged || len(ranges.Ranges) == 0 {
		return true
	}
	return ranges.Contains(line)
}

func firstChangedLine(ranges core.ChangedLineRanges) int {
	if ranges.AllChanged || len(ranges.Ranges) == 0 {
		return 1
	}
	best := ranges.Ranges[0][0]
	for _, r := range ranges.Ranges[1:] {
		if r[0] < best {
			best = r[0]
		}
	}
	if best < 1 {
		return 1
	}
	return best
}

func countWord(text string, word string) int {
	if text == "" || word == "" {
		return 0
	}
	return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(word)+`\b`).FindAllStringIndex(text, -1))
}

func isAdapterPath(rel string) bool {
	lower := strings.ToLower(normalizePath(rel))
	return hasPathSegment(lower, "adapter") || hasPathSegment(lower, "adapters")
}

func leadingSpaces(line string) int {
	count := 0
	for _, r := range line {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}

func isPublicDeclarationLine(line string, rel string) bool {
	ext := strings.ToLower(filepath.Ext(rel))
	if ext != "" {
		for _, pattern := range publicDeclPatternsByExtension[ext] {
			if pattern.MatchString(line) {
				return true
			}
		}
		return false
	}
	for _, patterns := range publicDeclPatternsByExtension {
		for _, pattern := range patterns {
			if pattern.MatchString(line) {
				return true
			}
		}
	}
	return false
}

func sortFunctions(functions []helperFunction) {
	sort.Slice(functions, func(i, j int) bool {
		if functions[i].path != functions[j].path {
			return functions[i].path < functions[j].path
		}
		if functions[i].line != functions[j].line {
			return functions[i].line < functions[j].line
		}
		return functions[i].name < functions[j].name
	})
}
