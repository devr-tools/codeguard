package quality

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	pythonToolchainImportPattern          = regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z_][\w.]*)(?:\s+as\s+\w+)?`)
	pythonToolchainFromPattern            = regexp.MustCompile(`(?m)^\s*from\s+([A-Za-z_][\w.]*|\.+[A-Za-z_][\w.]*)\s+import\s+`)
	pythonToolchainFromDotImportPattern   = regexp.MustCompile(`(?m)^\s*from\s+(\.+)\s+import\s+([A-Za-z_][\w]*(?:\s+as\s+\w+)?(?:\s*,\s*[A-Za-z_][\w]*(?:\s+as\s+\w+)?)*)`)
	pythonToolchainDynamicImportPattern   = regexp.MustCompile(`(?:importlib\.import_module|__import__)\(\s*["']([A-Za-z_][\w.]*)["']`)
	pythonToolchainUnknownDynamicPattern  = regexp.MustCompile(`(?:importlib\.import_module|__import__)\s*\(\s*([^'"\s])`)
	pythonToolchainEntryPointValuePattern = regexp.MustCompile(`["']([A-Za-z_][\w.]*)(?::[A-Za-z_][\w.]*)?["']`)
	pythonToolchainAllAssignmentPattern   = regexp.MustCompile(`(?s)__all__(?:\s*:[^=]+)?\s*=\s*(?:\[[^\]]*\]|\([^\)]*\)|\{[^\}]*\}|[^\n]+)`)
	pythonToolchainVultureTextPattern     = regexp.MustCompile(`^(.+\.py):(\d+):\s+unused\s+([A-Za-z_][\w -]*)\s+['"]?([^'"]*)['"]?(?:\s+\((\d+)% confidence\))?`)
)

type pythonToolchainModule struct {
	module        string
	rel           string
	edges         []string
	live          bool
	dynamicImport bool
}

func pythonToolchainDeadCodeFindings(env support.Context, target core.TargetConfig) []core.Finding {
	cfg := env.Config.Checks.QualityRules.DeadCode
	if !goToolchainDeadCodeEnabled(cfg) || !isPythonQualityTarget(target) {
		return nil
	}
	level := goToolchainDeadCodeLevel(cfg)
	modules := pythonToolchainModules(env, target, cfg)
	findings := pythonToolchainReportFindings(env, target, cfg, modules, level)
	entrypoints := pythonToolchainEntrypoints(env, target, cfg.Python.Entrypoints)
	if len(entrypoints) == 0 {
		if len(findings) > 0 {
			return findings
		}
		return []core.Finding{goToolchainDeadCodeDiagnostic(env, goToolchainDeadCodeLevel(cfg), "quality_rules.dead_code.python.entrypoints must name at least one Python entrypoint file, module, or glob")}
	}
	reachable := pythonToolchainReachable(modules, entrypoints)
	for _, module := range pythonToolchainSortedModules(modules) {
		node := modules[module]
		if reachable[module] || node.live {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     goToolchainDeadCodeRuleID,
			Level:      level,
			Path:       node.rel,
			Line:       1,
			Column:     1,
			Confidence: core.ConfidenceLow,
			Message: fmt.Sprintf(
				"Python module %q is outside the static import graph for configured entrypoints (%s)",
				module, strings.Join(entrypoints, ", "),
			),
			Metadata: map[string]string{
				"language": "python",
				"module":   module,
			},
		}))
	}
	return findings
}

type pythonToolchainReportIssue struct {
	Path       string
	Line       int
	Kind       string
	Name       string
	Confidence int
	Source     string
}

func pythonToolchainReportFindings(env support.Context, target core.TargetConfig, cfg core.QualityDeadCodeConfig, modules map[string]pythonToolchainModule, level string) []core.Finding {
	issues := pythonToolchainReportIssues(env, target, cfg)
	if len(issues) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0, len(issues))
	seen := map[string]struct{}{}
	for _, issue := range issues {
		rel := filepath.ToSlash(issue.Path)
		moduleName := pythonToolchainModuleName(rel)
		node, ok := modules[moduleName]
		if !ok || node.live || pythonToolchainReportIssueIgnored(rel, issue, cfg) {
			continue
		}
		key := fmt.Sprintf("%s:%d:%s:%s", rel, issue.Line, issue.Kind, issue.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     goToolchainDeadCodeRuleID,
			Level:      level,
			Path:       rel,
			Line:       issue.Line,
			Column:     1,
			Confidence: core.ConfidenceHigh,
			Message: fmt.Sprintf(
				"Python trusted dead-code report %q marks %s %q as unused",
				issue.Source, issue.Kind, issue.Name,
			),
			Metadata: map[string]string{
				"language": "python",
				"module":   moduleName,
				"kind":     issue.Kind,
				"evidence": "trusted-report",
			},
		}))
	}
	return findings
}

func pythonToolchainReportIssues(env support.Context, target core.TargetConfig, cfg core.QualityDeadCodeConfig) []pythonToolchainReportIssue {
	issues := make([]pythonToolchainReportIssue, 0)
	for _, report := range cfg.Python.Reports {
		rel := filepath.ToSlash(strings.TrimSpace(report))
		if rel == "" || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") {
			continue
		}
		data, ok := pythonToolchainReadReport(env, target, rel)
		if !ok {
			continue
		}
		issues = append(issues, pythonToolchainJSONReportIssues(rel, data)...)
		issues = append(issues, pythonToolchainVultureTextIssues(rel, string(data))...)
	}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Path != issues[j].Path {
			return issues[i].Path < issues[j].Path
		}
		if issues[i].Line != issues[j].Line {
			return issues[i].Line < issues[j].Line
		}
		return issues[i].Name < issues[j].Name
	})
	return issues
}

func pythonToolchainReadReport(env support.Context, target core.TargetConfig, rel string) ([]byte, bool) {
	if env.ReadTargetFile != nil {
		data, err := env.ReadTargetFile(target, rel)
		if err == nil {
			return data, true
		}
	}
	data, err := os.ReadFile(filepath.Join(target.Path, filepath.FromSlash(rel)))
	return data, err == nil
}

func pythonToolchainJSONReportIssues(source string, data []byte) []pythonToolchainReportIssue {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	records := pythonToolchainJSONRecords(raw)
	issues := make([]pythonToolchainReportIssue, 0, len(records))
	for _, record := range records {
		issue, ok := pythonToolchainJSONReportIssue(source, record)
		if ok {
			issues = append(issues, issue)
		}
	}
	return issues
}

func pythonToolchainJSONRecords(raw any) []map[string]any {
	switch value := raw.(type) {
	case []any:
		records := make([]map[string]any, 0, len(value))
		for _, item := range value {
			if record, ok := item.(map[string]any); ok {
				records = append(records, record)
			}
		}
		return records
	case map[string]any:
		for _, key := range []string{"items", "issues", "findings", "unused"} {
			if list, ok := value[key].([]any); ok {
				return pythonToolchainJSONRecords(list)
			}
		}
		return []map[string]any{value}
	default:
		return nil
	}
}

func pythonToolchainJSONReportIssue(source string, record map[string]any) (pythonToolchainReportIssue, bool) {
	rel := pythonToolchainReportString(record, "path", "file", "filename", "module_path")
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if rel == "" || !strings.HasSuffix(rel, ".py") || strings.HasPrefix(rel, "../") || filepath.IsAbs(rel) {
		return pythonToolchainReportIssue{}, false
	}
	kind := strings.ToLower(pythonToolchainReportString(record, "kind", "type", "code"))
	message := strings.ToLower(pythonToolchainReportString(record, "message", "reason", "description"))
	if !strings.Contains(kind+" "+message, "unused") && !strings.Contains(kind+" "+message, "dead") {
		return pythonToolchainReportIssue{}, false
	}
	name := pythonToolchainReportString(record, "name", "symbol", "object")
	if strings.TrimSpace(name) == "" {
		name = strings.TrimSpace(kind)
	}
	confidence := pythonToolchainReportInt(record, "confidence", "confidence_percent")
	return pythonToolchainReportIssue{
		Path:       rel,
		Line:       max(1, pythonToolchainReportInt(record, "line", "lineno", "line_number")),
		Kind:       pythonToolchainReportKind(kind, message),
		Name:       name,
		Confidence: confidence,
		Source:     source,
	}, true
}

func pythonToolchainVultureTextIssues(source string, data string) []pythonToolchainReportIssue {
	issues := make([]pythonToolchainReportIssue, 0)
	for _, line := range strings.Split(data, "\n") {
		match := pythonToolchainVultureTextPattern.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		lineNumber, _ := strconv.Atoi(match[2])
		confidence, _ := strconv.Atoi(match[5])
		issues = append(issues, pythonToolchainReportIssue{
			Path:       filepath.ToSlash(strings.TrimSpace(match[1])),
			Line:       max(1, lineNumber),
			Kind:       pythonToolchainReportKind(strings.TrimSpace(match[3]), ""),
			Name:       strings.TrimSpace(match[4]),
			Confidence: confidence,
			Source:     source,
		})
	}
	return issues
}

func pythonToolchainReportIssueIgnored(rel string, issue pythonToolchainReportIssue, cfg core.QualityDeadCodeConfig) bool {
	if issue.Confidence > 0 && issue.Confidence < 80 {
		return true
	}
	if !strings.HasSuffix(rel, ".py") || pythonToolchainTestPath(rel) {
		return true
	}
	for _, pattern := range cfg.Python.IgnorePaths {
		if support.PathMatchesPattern(pattern, rel) {
			return true
		}
	}
	return false
}

func pythonToolchainReportString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := record[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			return typed
		case fmt.Stringer:
			return typed.String()
		}
	}
	return ""
}

func pythonToolchainReportInt(record map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := record[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int(typed)
		case int:
			return typed
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(typed, "%")))
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}

func pythonToolchainReportKind(kind string, message string) string {
	text := strings.ToLower(kind + " " + message)
	switch {
	case strings.Contains(text, "class"):
		return "class"
	case strings.Contains(text, "method"):
		return "method"
	case strings.Contains(text, "function"):
		return "function"
	case strings.Contains(text, "import"):
		return "import"
	case strings.Contains(text, "module"), strings.Contains(text, "file"):
		return "module"
	default:
		return strings.TrimSpace(kind)
	}
}

func isPythonQualityTarget(target core.TargetConfig) bool {
	switch support.NormalizedLanguage(target.Language) {
	case "python", "py":
		return true
	default:
		return false
	}
}

func pythonToolchainEntrypoints(env support.Context, target core.TargetConfig, configured []string) []string {
	patterns := append([]string(nil), configured...)
	if len(patterns) == 0 {
		patterns = append(patterns, target.Entrypoints...)
	}
	files := goTargetFileList(env, target)
	seen := map[string]struct{}{}
	entrypoints := make([]string, 0)
	for _, pattern := range patterns {
		trimmed := filepath.ToSlash(strings.TrimSpace(pattern))
		if trimmed == "" {
			continue
		}
		if strings.HasSuffix(trimmed, ".py") || strings.ContainsAny(trimmed, "*?[") {
			for _, rel := range files {
				if strings.HasSuffix(rel, ".py") && support.PathMatchesPattern(trimmed, rel) {
					module := pythonToolchainModuleName(rel)
					if _, ok := seen[module]; !ok {
						seen[module] = struct{}{}
						entrypoints = append(entrypoints, module)
					}
				}
			}
			continue
		}
		module := strings.Trim(strings.ReplaceAll(trimmed, "/", "."), ".")
		if module != "" {
			if _, ok := seen[module]; !ok {
				seen[module] = struct{}{}
				entrypoints = append(entrypoints, module)
			}
		}
	}
	for _, module := range pythonToolchainDiscoveredEntrypoints(env, target) {
		if _, ok := seen[module]; ok {
			continue
		}
		seen[module] = struct{}{}
		entrypoints = append(entrypoints, module)
	}
	sort.Strings(entrypoints)
	return entrypoints
}

func pythonToolchainModules(env support.Context, target core.TargetConfig, cfg core.QualityDeadCodeConfig) map[string]pythonToolchainModule {
	includeTests := cfg.IncludeTests != nil && *cfg.IncludeTests
	nodes := map[string]pythonToolchainModule{}
	env.VisitTargetFiles(target, func(rel string) bool {
		if !strings.HasSuffix(rel, ".py") {
			return false
		}
		if !includeTests && pythonToolchainTestPath(rel) {
			return false
		}
		if pythonToolchainVendorPath(rel) {
			return false
		}
		for _, pattern := range cfg.Python.IgnorePaths {
			if support.PathMatchesPattern(pattern, rel) {
				return false
			}
		}
		return true
	}, func(rel string, data []byte) {
		source := string(data)
		if pythonToolchainGenerated(source) {
			return
		}
		module := pythonToolchainModuleName(rel)
		nodes[module] = pythonToolchainModule{
			module:        module,
			rel:           filepath.ToSlash(rel),
			live:          pythonToolchainConservativeLive(rel, source),
			dynamicImport: pythonToolchainHasUnknownDynamicImport(source),
		}
	})
	env.VisitTargetFiles(target, func(rel string) bool {
		_, ok := nodes[pythonToolchainModuleName(rel)]
		return ok
	}, func(rel string, data []byte) {
		module := pythonToolchainModuleName(rel)
		node := nodes[module]
		source := string(data)
		node.edges = pythonToolchainImportEdges(module, source, nodes)
		nodes[module] = node
		for _, exported := range pythonToolchainPackageExportEdges(module, node.rel, source, nodes) {
			exportedNode := nodes[exported]
			exportedNode.live = true
			nodes[exported] = exportedNode
		}
	})
	pythonToolchainMarkUnknownDynamicImportsLive(nodes)
	return nodes
}

func pythonToolchainImportEdges(module string, source string, nodes map[string]pythonToolchainModule) []string {
	seen := map[string]struct{}{}
	add := func(candidate string) {
		candidate = strings.Trim(candidate, ".")
		for candidate != "" {
			if _, ok := nodes[candidate]; ok {
				seen[candidate] = struct{}{}
				return
			}
			if idx := strings.LastIndex(candidate, "."); idx >= 0 {
				candidate = candidate[:idx]
				continue
			}
			return
		}
	}
	for _, match := range pythonToolchainImportPattern.FindAllStringSubmatch(source, -1) {
		add(match[1])
	}
	for _, match := range pythonToolchainFromPattern.FindAllStringSubmatch(source, -1) {
		name := match[1]
		if strings.HasPrefix(name, ".") {
			name = pythonToolchainResolveRelativeImport(module, name)
		}
		add(name)
	}
	for _, match := range pythonToolchainFromDotImportPattern.FindAllStringSubmatch(source, -1) {
		base := pythonToolchainResolveRelativeImport(module, match[1])
		for _, imported := range strings.Split(match[2], ",") {
			name := pythonToolchainImportedName(imported)
			if name != "" {
				add(strings.Trim(base+"."+name, "."))
			}
		}
	}
	for _, match := range pythonToolchainDynamicImportPattern.FindAllStringSubmatch(source, -1) {
		add(match[1])
	}
	edges := make([]string, 0, len(seen))
	for edge := range seen {
		edges = append(edges, edge)
	}
	sort.Strings(edges)
	return edges
}

func pythonToolchainReachable(nodes map[string]pythonToolchainModule, entrypoints []string) map[string]bool {
	reachable := map[string]bool{}
	queue := append([]string(nil), entrypoints...)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if reachable[current] {
			continue
		}
		node, ok := nodes[current]
		if !ok {
			continue
		}
		reachable[current] = true
		for _, edge := range node.edges {
			if !reachable[edge] {
				queue = append(queue, edge)
			}
		}
	}
	return reachable
}

func pythonToolchainModuleName(rel string) string {
	normalized := strings.TrimSuffix(filepath.ToSlash(rel), ".py")
	if strings.HasSuffix(normalized, "/__init__") {
		normalized = strings.TrimSuffix(normalized, "/__init__")
	}
	return strings.Trim(strings.ReplaceAll(normalized, "/", "."), ".")
}

func pythonToolchainResolveRelativeImport(module string, name string) string {
	level := 0
	for level < len(name) && name[level] == '.' {
		level++
	}
	baseParts := strings.Split(module, ".")
	if len(baseParts) > 0 && baseParts[len(baseParts)-1] != "__init__" {
		baseParts = baseParts[:len(baseParts)-1]
	}
	if level > 1 {
		drop := level - 1
		if drop >= len(baseParts) {
			baseParts = nil
		} else {
			baseParts = baseParts[:len(baseParts)-drop]
		}
	}
	suffix := strings.TrimPrefix(name, strings.Repeat(".", level))
	if suffix != "" {
		baseParts = append(baseParts, suffix)
	}
	return strings.Join(baseParts, ".")
}

func pythonToolchainSortedModules(nodes map[string]pythonToolchainModule) []string {
	modules := make([]string, 0, len(nodes))
	for module := range nodes {
		modules = append(modules, module)
	}
	sort.Strings(modules)
	return modules
}

func pythonToolchainAlwaysLive(rel string) bool {
	base := path.Base(filepath.ToSlash(rel))
	return base == "__init__.py"
}

func pythonToolchainConservativeLive(rel string, source string) bool {
	if pythonToolchainAlwaysLive(rel) {
		return true
	}
	normalized := filepath.ToSlash(rel)
	base := path.Base(normalized)
	switch base {
	case "manage.py", "wsgi.py", "asgi.py", "urls.py", "views.py", "routes.py", "tasks.py", "signals.py", "admin.py", "apps.py":
		return true
	}
	return strings.Contains(source, "importlib.import_module") ||
		strings.Contains(source, "__import__") ||
		strings.Contains(source, "@app.route") ||
		strings.Contains(source, "@router.") ||
		strings.Contains(source, "@shared_task") ||
		strings.Contains(source, "@celery.task") ||
		strings.Contains(source, "@receiver(") ||
		strings.Contains(source, "urlpatterns") ||
		strings.Contains(source, "FastAPI(") ||
		strings.Contains(source, "Flask(") ||
		strings.Contains(source, "__all__")
}

func pythonToolchainPackageExportEdges(module string, rel string, source string, nodes map[string]pythonToolchainModule) []string {
	if path.Base(filepath.ToSlash(rel)) != "__init__.py" {
		return nil
	}
	seen := map[string]struct{}{}
	add := func(candidate string) {
		if _, ok := nodes[candidate]; ok {
			seen[candidate] = struct{}{}
		}
	}
	for _, edge := range pythonToolchainImportEdges(module, source, nodes) {
		add(edge)
	}
	for _, name := range pythonToolchainQuotedAllNames(source) {
		add(strings.Trim(module+"."+name, "."))
	}
	edges := make([]string, 0, len(seen))
	for edge := range seen {
		edges = append(edges, edge)
	}
	sort.Strings(edges)
	return edges
}

func pythonToolchainTestPath(rel string) bool {
	normalized := strings.ToLower(filepath.ToSlash(rel))
	base := strings.TrimSuffix(path.Base(normalized), ".py")
	return strings.Contains(normalized, "/test/") ||
		strings.Contains(normalized, "/tests/") ||
		strings.Contains(normalized, "/fixtures/") ||
		strings.Contains(normalized, "/testdata/") ||
		strings.HasPrefix(base, "test_") ||
		strings.HasSuffix(base, "_test")
}

func pythonToolchainVendorPath(rel string) bool {
	normalized := strings.ToLower(filepath.ToSlash(rel))
	return strings.HasPrefix(normalized, "vendor/") ||
		strings.Contains(normalized, "/vendor/") ||
		strings.HasPrefix(normalized, "third_party/") ||
		strings.Contains(normalized, "/third_party/") ||
		strings.HasPrefix(normalized, "third-party/") ||
		strings.Contains(normalized, "/third-party/")
}

func pythonToolchainGenerated(source string) bool {
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "Code generated") && strings.Contains(trimmed, "DO NOT EDIT") {
			return true
		}
		if !strings.HasPrefix(trimmed, "#") {
			break
		}
	}
	return false
}

func pythonToolchainImportedName(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.Index(value, " as "); idx >= 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}

func pythonToolchainQuotedAllNames(source string) []string {
	if !strings.Contains(source, "__all__") {
		return nil
	}
	names := make([]string, 0)
	for _, assignment := range pythonToolchainAllAssignmentPattern.FindAllString(source, -1) {
		for _, match := range pythonToolchainEntryPointValuePattern.FindAllStringSubmatch(assignment, -1) {
			if strings.Contains(match[1], ".") {
				continue
			}
			names = append(names, match[1])
		}
	}
	return names
}

func pythonToolchainHasUnknownDynamicImport(source string) bool {
	return pythonToolchainUnknownDynamicPattern.MatchString(source)
}

func pythonToolchainMarkUnknownDynamicImportsLive(nodes map[string]pythonToolchainModule) {
	packages := map[string]struct{}{}
	markAll := false
	for module, node := range nodes {
		if !node.dynamicImport {
			continue
		}
		pkg := pythonToolchainContainingPackage(module)
		if pkg == "" {
			markAll = true
			continue
		}
		packages[pkg] = struct{}{}
	}
	if len(packages) == 0 && !markAll {
		return
	}
	for module, node := range nodes {
		if markAll || pythonToolchainPackageCovered(module, packages) {
			node.live = true
			nodes[module] = node
		}
	}
}

func pythonToolchainContainingPackage(module string) string {
	if idx := strings.LastIndex(module, "."); idx >= 0 {
		return module[:idx]
	}
	return ""
}

func pythonToolchainPackageCovered(module string, packages map[string]struct{}) bool {
	for pkg := range packages {
		if module == pkg || strings.HasPrefix(module, pkg+".") {
			return true
		}
	}
	return false
}

func pythonToolchainDiscoveredEntrypoints(env support.Context, target core.TargetConfig) []string {
	seen := map[string]struct{}{}
	add := func(module string) {
		module = strings.Trim(strings.Split(module, ":")[0], ". ")
		if module == "" {
			return
		}
		seen[module] = struct{}{}
	}
	env.VisitTargetFiles(target, func(rel string) bool {
		normalized := filepath.ToSlash(rel)
		switch path.Base(normalized) {
		case "pyproject.toml", "setup.cfg":
			return true
		case "manage.py", "wsgi.py", "asgi.py", "app.py", "main.py":
			return strings.HasSuffix(normalized, ".py")
		default:
			return false
		}
	}, func(rel string, data []byte) {
		switch path.Base(filepath.ToSlash(rel)) {
		case "pyproject.toml":
			for _, module := range pythonToolchainPyprojectEntrypoints(string(data)) {
				add(module)
			}
		case "setup.cfg":
			for _, module := range pythonToolchainSetupCfgEntrypoints(string(data)) {
				add(module)
			}
		default:
			add(pythonToolchainModuleName(rel))
		}
	})
	entrypoints := make([]string, 0, len(seen))
	for module := range seen {
		entrypoints = append(entrypoints, module)
	}
	sort.Strings(entrypoints)
	return entrypoints
}

func pythonToolchainPyprojectEntrypoints(source string) []string {
	entrypoints := make([]string, 0)
	section := ""
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(strings.Split(line, "#")[0])
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.Trim(trimmed, "[] ")
			continue
		}
		switch section {
		case "project.scripts", "project.gui-scripts", "tool.poetry.scripts":
			for _, match := range pythonToolchainEntryPointValuePattern.FindAllStringSubmatch(trimmed, -1) {
				entrypoints = append(entrypoints, strings.Split(match[1], ":")[0])
			}
		}
	}
	return entrypoints
}

func pythonToolchainSetupCfgEntrypoints(source string) []string {
	entrypoints := make([]string, 0)
	section := ""
	group := ""
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(strings.Split(line, "#")[0])
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.Trim(trimmed, "[] ")
			group = ""
			continue
		}
		if section != "options.entry_points" {
			continue
		}
		if strings.HasSuffix(trimmed, "=") {
			group = strings.TrimSpace(strings.TrimSuffix(trimmed, "="))
			continue
		}
		if group != "console_scripts" && group != "gui_scripts" {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		module := strings.TrimSpace(strings.Split(parts[1], ":")[0])
		if module != "" {
			entrypoints = append(entrypoints, module)
		}
	}
	return entrypoints
}
