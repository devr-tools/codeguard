package quality

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const (
	goToolchainDeadCodeRuleID     = "quality.dead-code.toolchain"
	goToolchainCommandOutputLimit = 1 << 20
)

type goToolchainCandidate struct {
	rel        string
	importPath string
	name       string
	symbol     string
	line       int
	column     int
}

func goToolchainDeadCodeFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	cfg := env.Config.Checks.QualityRules.DeadCode
	if !goToolchainDeadCodeEnabled(cfg) || !isGoQualityTarget(target) {
		return nil
	}
	if cfg.Go.Linker != nil && !*cfg.Go.Linker {
		return nil
	}
	level := goToolchainDeadCodeLevel(cfg)
	entrypoints := goToolchainEntrypoints(env, target, cfg.Go.Entrypoints)
	if len(entrypoints) == 0 {
		return []core.Finding{goToolchainDeadCodeDiagnostic(env, level, "quality_rules.dead_code.go.entrypoints must name at least one Go entrypoint package or file")}
	}
	modulePath := support.GoModulePath(target.Path)
	if modulePath == "" {
		return []core.Finding{goToolchainDeadCodeDiagnostic(env, level, "go.mod was not found for Go toolchain dead-code analysis")}
	}
	candidates, packages := goToolchainCandidates(env, target, modulePath, cfg.Go.IgnorePaths, cfg.Go.Packages)
	linked, err := linkedGoSymbols(ctx, target.Path, entrypoints)
	if err != nil {
		return []core.Finding{goToolchainDeadCodeDiagnostic(env, level, fmt.Sprintf("Go linker dependency analysis failed: %v", err))}
	}
	findings := make([]core.Finding, 0)
	findings = append(findings, goToolchainUnreachablePackageFindings(env, level, packages, linked, entrypoints)...)
	for _, candidate := range candidates {
		if !goToolchainPackageLinked(candidate.importPath, linked) || linked[candidate.symbol] {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     goToolchainDeadCodeRuleID,
			Level:      level,
			Path:       candidate.rel,
			Line:       candidate.line,
			Column:     candidate.column,
			Confidence: core.ConfidenceHigh,
			Message: fmt.Sprintf(
				"private Go function %q is absent from linker dependency graphs for configured entrypoints (%s)",
				candidate.name, strings.Join(entrypoints, ", "),
			),
			Metadata: map[string]string{
				"symbol":      candidate.symbol,
				"import_path": candidate.importPath,
			},
		}))
	}
	return findings
}

func goToolchainDeadCodeEnabled(cfg core.QualityDeadCodeConfig) bool {
	if cfg.Enabled == nil || !*cfg.Enabled {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
	case "", "toolchain":
		return true
	default:
		return false
	}
}

func goToolchainDeadCodeLevel(cfg core.QualityDeadCodeConfig) string {
	switch strings.ToLower(strings.TrimSpace(cfg.Level)) {
	case "fail":
		return "fail"
	default:
		return "warn"
	}
}

func goToolchainDeadCodeDiagnostic(env support.Context, level string, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  goToolchainDeadCodeRuleID,
		Level:   level,
		Message: message,
	})
}

func isGoQualityTarget(target core.TargetConfig) bool {
	switch support.NormalizedLanguage(target.Language) {
	case "", "go":
		return true
	default:
		return false
	}
}

func goToolchainEntrypoints(env support.Context, target core.TargetConfig, configured []string) []string {
	patterns := append([]string(nil), configured...)
	if len(patterns) == 0 {
		patterns = append(patterns, target.Entrypoints...)
	}
	seen := map[string]struct{}{}
	entrypoints := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		for _, pkg := range goToolchainPackagesForEntrypoint(env, target, pattern) {
			if _, ok := seen[pkg]; ok {
				continue
			}
			seen[pkg] = struct{}{}
			entrypoints = append(entrypoints, pkg)
		}
	}
	sort.Strings(entrypoints)
	return entrypoints
}

func goToolchainPackagesForEntrypoint(env support.Context, target core.TargetConfig, pattern string) []string {
	trimmed := filepath.ToSlash(strings.TrimSpace(pattern))
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, ".") || strings.Contains(trimmed, "...") {
		return []string{trimmed}
	}
	if strings.HasSuffix(trimmed, ".go") || strings.ContainsAny(trimmed, "*?[") {
		files := goTargetFileList(env, target)
		seen := map[string]struct{}{}
		packages := make([]string, 0)
		for _, rel := range files {
			if !strings.HasSuffix(rel, ".go") || !support.PathMatchesPattern(trimmed, rel) {
				continue
			}
			pkg := goPackagePattern(path.Dir(filepath.ToSlash(rel)))
			if _, ok := seen[pkg]; ok {
				continue
			}
			seen[pkg] = struct{}{}
			packages = append(packages, pkg)
		}
		sort.Strings(packages)
		return packages
	}
	return []string{goPackagePattern(trimmed)}
}

func goPackagePattern(dir string) string {
	dir = strings.Trim(filepath.ToSlash(dir), "/")
	if dir == "" || dir == "." {
		return "."
	}
	return "./" + dir
}

func goTargetFileList(env support.Context, target core.TargetConfig) []string {
	if env.ListTargetFiles != nil {
		files, err := env.ListTargetFiles(target)
		if err == nil {
			return files
		}
	}
	files := make([]string, 0)
	env.VisitTargetFiles(target, func(string) bool { return true }, func(rel string, _ []byte) {
		files = append(files, rel)
	})
	return files
}

type goToolchainPackage struct {
	importPath         string
	rel                string
	directiveSensitive bool
}

func goToolchainCandidates(env support.Context, target core.TargetConfig, modulePath string, ignorePaths []string, packagePatterns []string) ([]goToolchainCandidate, []goToolchainPackage) {
	candidates := make([]goToolchainCandidate, 0)
	packagesByImportPath := map[string]goToolchainPackage{}
	env.VisitTargetFiles(target, func(rel string) bool {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return false
		}
		if goToolchainPathIgnored(rel) {
			return false
		}
		if !goToolchainPackagePatternAllows(rel, packagePatterns) {
			return false
		}
		for _, pattern := range ignorePaths {
			if support.PathMatchesPattern(pattern, rel) {
				return false
			}
		}
		return true
	}, func(rel string, data []byte) {
		if goToolchainGeneratedFile(data) {
			return
		}
		fset, parsed, err := support.ParseGoSource(env, rel, data)
		if err != nil {
			return
		}
		if parsed.Name != nil && parsed.Name.Name == "main" {
			return
		}
		importPath := goImportPathForFile(modulePath, rel)
		if _, ok := packagesByImportPath[importPath]; !ok {
			packagesByImportPath[importPath] = goToolchainPackage{
				importPath: importPath,
				rel:        rel,
			}
		}
		if goToolchainDirectiveSensitive(data) {
			pkg := packagesByImportPath[importPath]
			pkg.directiveSensitive = true
			packagesByImportPath[importPath] = pkg
			return
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !goToolchainFunctionEligible(parsed, fn) {
				continue
			}
			pos := fset.Position(fn.Name.Pos())
			candidates = append(candidates, goToolchainCandidate{
				rel:        rel,
				importPath: importPath,
				name:       fn.Name.Name,
				symbol:     goToolchainSymbol(parsed, importPath, fn.Name.Name),
				line:       pos.Line,
				column:     pos.Column,
			})
		}
	})
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].rel != candidates[j].rel {
			return candidates[i].rel < candidates[j].rel
		}
		return candidates[i].name < candidates[j].name
	})
	packages := make([]goToolchainPackage, 0, len(packagesByImportPath))
	for _, pkg := range packagesByImportPath {
		packages = append(packages, pkg)
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].importPath < packages[j].importPath })
	return candidates, packages
}

func goToolchainPackagePatternAllows(rel string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	pkgPattern := goPackagePattern(path.Dir(filepath.ToSlash(rel)))
	for _, pattern := range patterns {
		trimmed := filepath.ToSlash(strings.TrimSpace(pattern))
		switch {
		case trimmed == "", trimmed == "./...":
			return true
		case strings.HasSuffix(trimmed, "/..."):
			prefix := strings.TrimSuffix(trimmed, "/...")
			if pkgPattern == prefix || strings.HasPrefix(pkgPattern, prefix+"/") {
				return true
			}
		case pkgPattern == trimmed || support.PathMatchesPattern(strings.TrimPrefix(trimmed, "./")+"/**", path.Dir(filepath.ToSlash(rel))):
			return true
		}
	}
	return false
}

func goToolchainUnreachablePackageFindings(env support.Context, level string, packages []goToolchainPackage, linked map[string]bool, entrypoints []string) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, pkg := range packages {
		if pkg.directiveSensitive || goToolchainPackageLinked(pkg.importPath, linked) {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     goToolchainDeadCodeRuleID,
			Level:      level,
			Path:       pkg.rel,
			Line:       1,
			Column:     1,
			Confidence: core.ConfidenceHigh,
			Message: fmt.Sprintf(
				"Go package %q is absent from linker dependency graphs for configured entrypoints (%s)",
				pkg.importPath, strings.Join(entrypoints, ", "),
			),
			Metadata: map[string]string{"import_path": pkg.importPath},
		}))
	}
	return findings
}

func goToolchainPackageLinked(importPath string, linked map[string]bool) bool {
	prefix := importPath + "."
	for symbol := range linked {
		if strings.HasPrefix(symbol, prefix) {
			return true
		}
	}
	return false
}

func goToolchainFunctionEligible(parsed *ast.File, fn *ast.FuncDecl) bool {
	if fn.Recv != nil || fn.Name == nil {
		return false
	}
	name := fn.Name.Name
	if name == "init" || name == "main" || name == "TestMain" || !startsLowercase(name) {
		return false
	}
	if parsed != nil && parsed.Name != nil && parsed.Name.Name == "main" {
		return false
	}
	return true
}

func goToolchainGeneratedFile(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "Code generated") && strings.Contains(trimmed, "DO NOT EDIT") {
			return true
		}
		if !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "/*") && !strings.HasPrefix(trimmed, "*") {
			return false
		}
	}
	return false
}

func goToolchainPathIgnored(rel string) bool {
	normalized := strings.ToLower(filepath.ToSlash(rel))
	return strings.HasPrefix(normalized, "vendor/") ||
		strings.Contains(normalized, "/vendor/") ||
		strings.HasPrefix(normalized, "third_party/") ||
		strings.Contains(normalized, "/third_party/") ||
		strings.HasPrefix(normalized, "third-party/") ||
		strings.Contains(normalized, "/third-party/") ||
		strings.HasPrefix(normalized, "testdata/") ||
		strings.Contains(normalized, "/testdata/")
}

func goToolchainDirectiveSensitive(data []byte) bool {
	source := string(data)
	return strings.Contains(source, "//go:linkname") ||
		strings.Contains(source, "//export ") ||
		strings.Contains(source, "import \"C\"")
}

func goImportPathForFile(modulePath string, rel string) string {
	dir := path.Dir(filepath.ToSlash(rel))
	if dir == "." {
		return modulePath
	}
	return modulePath + "/" + dir
}

func goToolchainSymbol(parsed *ast.File, importPath string, name string) string {
	if parsed != nil && parsed.Name != nil && parsed.Name.Name == "main" {
		return "main." + name
	}
	return importPath + "." + name
}

func linkedGoSymbols(ctx context.Context, targetDir string, entrypoints []string) (map[string]bool, error) {
	tmpDir, err := os.MkdirTemp("", "codeguard-go-dead-code-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	linked := map[string]bool{}
	env := goToolchainCommandEnv(tmpDir)
	for idx, entrypoint := range entrypoints {
		outputPath := filepath.Join(tmpDir, fmt.Sprintf("entrypoint-%d", idx))
		output, err := runGoToolchainCommand(ctx, targetDir, env, "go", "build", "-gcflags=all=-l", "-ldflags=-dumpdep", "-o", outputPath, entrypoint)
		if err != nil {
			if strings.TrimSpace(output) != "" {
				return nil, fmt.Errorf("%s: %w: %s", entrypoint, err, strings.TrimSpace(output))
			}
			return nil, fmt.Errorf("%s: %w", entrypoint, err)
		}
		for _, symbol := range parseGoLinkerDumpSymbols(output) {
			linked[symbol] = true
		}
	}
	return linked, nil
}

func goToolchainCommandEnv(tmpDir string) []string {
	env := make([]string, 0, len(os.Environ())+3)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "GOROOT=") || strings.HasPrefix(item, "GOCACHE=") {
			continue
		}
		env = append(env, item)
	}
	env = append(env,
		"GOCACHE="+filepath.Join(tmpDir, "gocache"),
		"GO111MODULE=on",
	)
	return env
}

func runGoToolchainCommand(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // fixed Go tool invocation; config only supplies validated package patterns
	cmd.Dir = dir
	cmd.Env = env
	var buf bytes.Buffer
	limited := newGoToolchainLimitedWriter(&buf, goToolchainCommandOutputLimit)
	cmd.Stdout = limited
	cmd.Stderr = limited
	err := cmd.Run()
	if limited.Truncated() {
		return "", fmt.Errorf("%s output exceeded %d bytes", name, goToolchainCommandOutputLimit)
	}
	return buf.String(), err
}

type goToolchainLimitedWriter struct {
	buf       *bytes.Buffer
	limit     int
	truncated bool
}

func newGoToolchainLimitedWriter(buf *bytes.Buffer, limit int) *goToolchainLimitedWriter {
	return &goToolchainLimitedWriter{buf: buf, limit: limit}
}

func (w *goToolchainLimitedWriter) Write(p []byte) (int, error) {
	if w.truncated {
		return len(p), nil
	}
	remaining := w.limit - w.buf.Len()
	if remaining <= 0 {
		w.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = w.buf.Write(p[:remaining])
		w.truncated = true
		return len(p), nil
	}
	_, _ = w.buf.Write(p)
	return len(p), nil
}

func (w *goToolchainLimitedWriter) Truncated() bool {
	return w.truncated
}

func parseGoLinkerDumpSymbols(output string) []string {
	seen := map[string]struct{}{}
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "# ") {
			continue
		}
		for _, part := range strings.Split(line, " -> ") {
			symbol := strings.TrimSpace(part)
			if symbol == "" || strings.HasPrefix(symbol, "go:") || strings.HasPrefix(symbol, "type:") {
				continue
			}
			seen[symbol] = struct{}{}
		}
	}
	symbols := make([]string, 0, len(seen))
	for symbol := range seen {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	return symbols
}
