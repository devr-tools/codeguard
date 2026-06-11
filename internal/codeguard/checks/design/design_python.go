package design

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	pythonImportPattern     = regexp.MustCompile(`^\s*import\s+([A-Za-z0-9_.,\s]+)`)
	pythonFromImportPattern = regexp.MustCompile(`^\s*from\s+([A-Za-z0-9_\.]+|\.)+\s+import\s+([A-Za-z0-9_.,\s*]+)`)
)

func pythonTargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	return env.ScanTargetFiles(target, "design", func(rel string) bool {
		return strings.EqualFold(filepath.Ext(rel), ".py")
	}, func(file string, data []byte) []core.Finding {
		return pythonFindingsForFile(env, target, file, data)
	})
}

func pythonFindingsForFile(env support.Context, target core.TargetConfig, file string, data []byte) []core.Finding {
	findings := genericPythonModuleNameFindings(env, file)
	if !isPublicPythonModule(file, target) {
		return findings
	}

	entrypoints := pythonEntrypointModules(target.Entrypoints)
	currentModule := pythonModuleName(file)
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for idx, line := range lines {
		lineNo := idx + 1
		importedModules, importedNames := pythonImportsForLine(currentModule, line)
		if len(importedModules) == 0 && len(importedNames) == 0 {
			continue
		}
		if importsPrivatePythonModule(importedModules, importedNames) {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.python.public-imports-private",
				Level:   "fail",
				Path:    file,
				Line:    lineNo,
				Column:  1,
				Message: "public Python module imports a private module",
			}))
		}
		if importsPythonEntrypoint(importedModules, entrypoints) {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.python.public-imports-cli",
				Level:   "fail",
				Path:    file,
				Line:    lineNo,
				Column:  1,
				Message: "public Python module imports a CLI or entrypoint module",
			}))
		}
	}

	return findings
}

func genericPythonModuleNameFindings(env support.Context, file string) []core.Finding {
	moduleName := strings.ToLower(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)))
	for _, forbidden := range env.Config.Checks.DesignRules.ForbiddenPackageNames {
		if strings.EqualFold(moduleName, forbidden) {
			return []core.Finding{env.NewFinding(support.FindingInput{
				RuleID:  "design.python.generic-module-name",
				Level:   "warn",
				Path:    file,
				Line:    1,
				Column:  1,
				Message: fmt.Sprintf("module name %q is too generic", moduleName),
			})}
		}
	}
	return nil
}

func isPublicPythonModule(file string, target core.TargetConfig) bool {
	slash := filepath.ToSlash(file)
	base := filepath.Base(slash)
	if strings.HasPrefix(base, "_") || strings.HasPrefix(slash, "tests/") || strings.Contains(slash, "/tests/") {
		return false
	}
	for _, entrypoint := range target.Entrypoints {
		if filepath.ToSlash(entrypoint) == slash {
			return false
		}
	}
	return true
}

func pythonEntrypointModules(paths []string) map[string]struct{} {
	modules := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		module := pythonModuleName(path)
		if module != "" {
			modules[module] = struct{}{}
		}
	}
	return modules
}

func pythonModuleName(path string) string {
	slash := filepath.ToSlash(path)
	slash = strings.TrimSuffix(slash, ".py")
	slash = strings.TrimSuffix(slash, "/__init__")
	slash = strings.TrimPrefix(slash, "./")
	return strings.ReplaceAll(slash, "/", ".")
}

func pythonImportsForLine(currentModule string, line string) ([]string, []string) {
	if match := pythonImportPattern.FindStringSubmatch(line); len(match) == 2 {
		parts := strings.Split(match[1], ",")
		modules := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			fields := strings.Fields(part)
			if len(fields) > 0 {
				modules = append(modules, fields[0])
			}
		}
		return modules, nil
	}

	if match := pythonFromImportPattern.FindStringSubmatch(line); len(match) == 3 {
		module := resolvePythonImportModule(currentModule, strings.TrimSpace(match[1]))
		names := strings.Split(match[2], ",")
		importedNames := make([]string, 0, len(names))
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			fields := strings.Fields(name)
			if len(fields) > 0 {
				importedNames = append(importedNames, fields[0])
			}
		}
		if module == "" {
			return nil, importedNames
		}
		return []string{module}, importedNames
	}

	return nil, nil
}

func resolvePythonImportModule(currentModule string, imported string) string {
	if !strings.HasPrefix(imported, ".") {
		return imported
	}

	dots := 0
	for dots < len(imported) && imported[dots] == '.' {
		dots++
	}
	remainder := strings.TrimPrefix(imported, strings.Repeat(".", dots))

	parts := strings.Split(currentModule, ".")
	if len(parts) == 0 {
		return remainder
	}
	limit := len(parts) - dots
	if limit < 0 {
		limit = 0
	}
	base := parts[:limit]
	if remainder != "" {
		base = append(base, remainder)
	}
	return strings.Join(base, ".")
}

func importsPrivatePythonModule(modules []string, names []string) bool {
	for _, module := range modules {
		for _, part := range strings.Split(module, ".") {
			if strings.HasPrefix(part, "_") {
				return true
			}
		}
	}
	for _, name := range names {
		if strings.HasPrefix(name, "_") {
			return true
		}
	}
	return false
}

func importsPythonEntrypoint(modules []string, entrypoints map[string]struct{}) bool {
	for _, module := range modules {
		if _, ok := entrypoints[module]; ok {
			return true
		}
	}
	return false
}
