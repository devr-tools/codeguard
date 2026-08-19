package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func maintainabilityDeltaFindings(env support.Context, target core.TargetConfig) []core.Finding {
	if env.Mode != core.ScanModeDiff || env.ReadBaseFile == nil {
		return nil
	}
	changed := changedFilesForTarget(env, target)
	if len(changed) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, rel := range changed {
		if !qualityPrecisionSupportsFile(target.Language, rel) {
			continue
		}
		current, ok := readCurrentTargetFile(env, target, rel)
		if !ok {
			continue
		}
		base, err := env.ReadBaseFile(target, rel)
		if err != nil {
			continue
		}
		line := deltaFindingLine(env, rel)
		findings = append(findings, publicSurfaceGrowthFinding(env, target, rel, base, current, line)...)
		findings = append(findings, dependencyGrowthFinding(env, target, rel, base, current, line)...)
	}
	return findings
}

func changedFilesForTarget(env support.Context, target core.TargetConfig) []string {
	seen := map[string]struct{}{}
	if env.ListChangedFiles != nil {
		if changed, err := env.ListChangedFiles(target); err == nil {
			for _, file := range changed {
				if file.Status == core.ChangedFileDeleted {
					continue
				}
				seen[filepath.ToSlash(file.Path)] = struct{}{}
			}
		}
	}
	if len(seen) == 0 && env.DiffScope != nil {
		for rel := range env.DiffScope() {
			seen[filepath.ToSlash(rel)] = struct{}{}
		}
	}
	paths := make([]string, 0, len(seen))
	for rel := range seen {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	return paths
}

func readCurrentTargetFile(env support.Context, target core.TargetConfig, rel string) ([]byte, bool) {
	if env.ReadTargetFile != nil {
		data, err := env.ReadTargetFile(target, rel)
		return data, err == nil
	}
	data, err := os.ReadFile(filepath.Join(target.Path, filepath.FromSlash(rel))) //nolint:gosec // rel comes from the scan's own changed-file list
	return data, err == nil
}

func deltaFindingLine(env support.Context, rel string) int {
	if env.DiffScope == nil {
		return 1
	}
	scope := env.DiffScope()[filepath.ToSlash(rel)]
	if scope.AllChanged {
		return 1
	}
	for _, r := range scope.Ranges {
		if r[0] > 0 {
			return r[0]
		}
	}
	return 1
}

func publicSurfaceGrowthFinding(env support.Context, target core.TargetConfig, rel string, base []byte, current []byte, line int) []core.Finding {
	baseCount := publicSurfaceCount(target.Language, rel, string(base))
	currentCount := publicSurfaceCount(target.Language, rel, string(current))
	if currentCount <= baseCount {
		return nil
	}
	return []core.Finding{precisionWarnFinding(env, maintainabilityPublicSurfaceGrowthID, rel, line,
		fmt.Sprintf("public surface grew from %d to %d symbols in this file; keep new exported API intentional", baseCount, currentCount),
		core.ConfidenceHigh)}
}

func dependencyGrowthFinding(env support.Context, target core.TargetConfig, rel string, base []byte, current []byte, line int) []core.Finding {
	baseCount := len(dependencySet(target.Language, rel, string(base)))
	currentCount := len(dependencySet(target.Language, rel, string(current)))
	if currentCount <= baseCount {
		return nil
	}
	return []core.Finding{precisionWarnFinding(env, maintainabilityDependencyGrowthID, rel, line,
		fmt.Sprintf("direct dependencies grew from %d to %d in this file; verify the added dependency surface is necessary", baseCount, currentCount),
		core.ConfidenceHigh)}
}

func qualityPrecisionSupportsFile(language string, rel string) bool {
	switch support.NormalizedLanguage(language) {
	case "", "go":
		return strings.HasSuffix(rel, ".go")
	case "python":
		return strings.HasSuffix(rel, ".py")
	case "typescript", "javascript":
		return isTypeScriptLikeFile(rel)
	case "c++", "cpp":
		return strings.HasSuffix(rel, ".cpp") || strings.HasSuffix(rel, ".cc") || strings.HasSuffix(rel, ".cxx") ||
			strings.HasSuffix(rel, ".hpp") || strings.HasSuffix(rel, ".hh") || strings.HasSuffix(rel, ".h")
	default:
		return false
	}
}

func publicSurfaceCount(language string, rel string, source string) int {
	switch support.NormalizedLanguage(language) {
	case "", "go":
		return len(goPublicDeclPattern.FindAllStringSubmatch(source, -1))
	case "python":
		count := 0
		for _, match := range pythonPublicDeclPattern.FindAllStringSubmatch(source, -1) {
			name := firstNonEmptyString(match[1], match[2])
			if name != "" && !strings.HasPrefix(name, "_") {
				count++
			}
		}
		return count
	case "typescript", "javascript":
		return len(tsPublicDeclPattern.FindAllStringSubmatch(source, -1))
	case "c++", "cpp":
		if !strings.HasSuffix(rel, ".h") && !strings.HasSuffix(rel, ".hh") && !strings.HasSuffix(rel, ".hpp") {
			return 0
		}
		return len(cppPublicDeclPattern.FindAllStringSubmatch(source, -1))
	default:
		return 0
	}
}

func dependencySet(language string, rel string, source string) map[string]struct{} {
	deps := map[string]struct{}{}
	switch support.NormalizedLanguage(language) {
	case "", "go":
		if fset, parsed, err := support.ParseGoSource(support.Context{}, rel, []byte(source)); err == nil {
			_ = fset
			for _, imp := range parsed.Imports {
				deps[strings.Trim(imp.Path.Value, `"`)] = struct{}{}
			}
		}
	case "python":
		for _, imp := range support.ParsePython(source).Imports {
			deps[firstNonEmptyString(imp.Module, imp.Name, imp.Alias)] = struct{}{}
		}
	case "typescript", "javascript":
		for _, imp := range support.ParseCLike(source, support.CLikeTypeScript).Imports {
			deps[imp.Module] = struct{}{}
		}
	case "c++", "cpp":
		for _, match := range cppIncludePattern.FindAllStringSubmatch(source, -1) {
			deps[match[1]] = struct{}{}
		}
	}
	return deps
}

func isQualityFixturePath(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(path))
	if strings.Contains(normalized, "/testdata/") || strings.Contains(normalized, "/fixtures/") ||
		strings.Contains(normalized, "/__fixtures__/") || strings.Contains(normalized, "/__tests__/") ||
		strings.Contains(normalized, "/tests/") {
		return true
	}
	return strings.HasSuffix(normalized, "_test.go") || strings.HasSuffix(normalized, "_test.py") ||
		strings.HasSuffix(normalized, ".test.ts") || strings.HasSuffix(normalized, ".spec.ts") ||
		strings.HasSuffix(normalized, ".test.tsx") || strings.HasSuffix(normalized, ".spec.tsx") ||
		strings.HasSuffix(normalized, ".test.js") || strings.HasSuffix(normalized, ".spec.js") ||
		strings.HasSuffix(normalized, ".test.jsx") || strings.HasSuffix(normalized, ".spec.jsx")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
