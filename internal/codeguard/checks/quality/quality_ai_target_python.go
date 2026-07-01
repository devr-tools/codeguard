package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	pythonImportStmtPattern     = regexp.MustCompile(`^\s*import\s+(.+)$`)
	pythonFromImportStmtPattern = regexp.MustCompile(`^\s*from\s+([A-Za-z_][\w.]*)\s+import\b`)
	pythonModuleNamePattern     = regexp.MustCompile(`^[A-Za-z_][\w.]*$`)
)

func pythonAITargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	files := aiTargetSourceFiles(env, target, ".py")
	if len(files) == 0 {
		return nil
	}
	catalog := readPythonDependencyCatalog(target.Path)
	localModules := pythonLocalModuleNames(target.Path, files)
	repoErrorStyle := pythonRepoErrorStyle(target.Path, files)
	repoNaming := dominantNamingConvention(target.Path, files, pythonDeclaredNames)
	findings := make([]core.Finding, 0)
	for _, rel := range files {
		findings = append(findings, pythonFileAIQualityFindings(env, target.Path, rel, pythonFileScanInput{
			catalog:      catalog,
			localModules: localModules,
			errorStyle:   repoErrorStyle,
			naming:       repoNaming,
		})...)
	}
	return findings
}

type pythonFileScanInput struct {
	catalog      pythonDependencyCatalog
	localModules map[string]struct{}
	errorStyle   pythonErrorStyleSummary
	naming       string
}

func pythonFileAIQualityFindings(env support.Context, root string, rel string, input pythonFileScanInput) []core.Finding {
	data, err := os.ReadFile(filepath.Join(root, rel)) //nolint:gosec // file under the scan-target root
	if err != nil {
		return nil
	}
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	findings := make([]core.Finding, 0)
	if aiCheckEnabled(env.Config.Checks.QualityRules.AIChecks.HallucinatedImport) {
		findings = append(findings, pythonImportFindings(env, root, rel, source, input)...)
	}
	if aiCheckEnabled(env.Config.Checks.QualityRules.AIChecks.DeadCode) {
		findings = append(findings, pythonDeadCodeFindings(env, rel, source)...)
		findings = append(findings, pythonUnusedPrivateFunctionFindings(env, rel, source)...)
	}
	if aiCheckEnabled(env.Config.Checks.QualityRules.AIChecks.ErrorStyleDrift) {
		findings = append(findings, pythonErrorStyleDriftFindings(env, rel, source, input.errorStyle)...)
	}
	if aiCheckEnabled(env.Config.Checks.QualityRules.AIChecks.NamingDrift) {
		findings = append(findings, namingDriftFinding(env, rel, source, input.naming, pythonDeclaredNames)...)
	}
	return findings
}

// pythonLocalModuleNames collects top-level module and package names that
// exist on disk so that local imports resolve without manifests.
func pythonLocalModuleNames(_ string, files []string) map[string]struct{} {
	names := map[string]struct{}{}
	for _, rel := range files {
		slash := filepath.ToSlash(rel)
		segments := strings.Split(slash, "/")
		base := strings.TrimSuffix(segments[len(segments)-1], ".py")
		if base != "" && base != "__init__" {
			names[base] = struct{}{}
		}
		// Every ancestor directory of a Python file can act as a package or
		// namespace-package root for imports inside the repository.
		for _, segment := range segments[:len(segments)-1] {
			if segment != "" {
				names[segment] = struct{}{}
			}
		}
	}
	return names
}

func pythonImportFindings(env support.Context, root string, file string, source string, input pythonFileScanInput) []core.Finding {
	findings := make([]core.Finding, 0)
	for idx, line := range strings.Split(source, "\n") {
		for _, module := range pythonImportedModules(line) {
			if pythonImportResolvable(root, file, module, input.catalog, input.localModules) {
				continue
			}
			findings = append(findings, warnFinding(env, "quality.ai.hallucinated-import", file, idx+1, 1,
				fmt.Sprintf("import %q does not resolve against the standard library, declared dependencies, or local modules", module)))
		}
	}
	return findings
}

// pythonImportedModules extracts dotted module paths imported by a single
// source line, handling "import a.b as c, d" and "from x.y import z".
func pythonImportedModules(line string) []string {
	if match := pythonFromImportStmtPattern.FindStringSubmatch(line); match != nil {
		return []string{match[1]}
	}
	match := pythonImportStmtPattern.FindStringSubmatch(line)
	if match == nil {
		return nil
	}
	modules := make([]string, 0, 1)
	for _, clause := range strings.Split(match[1], ",") {
		fields := strings.Fields(strings.TrimSpace(clause))
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if !pythonModuleNamePattern.MatchString(name) {
			continue
		}
		modules = append(modules, name)
	}
	return modules
}

func pythonImportResolvable(root string, file string, module string, catalog pythonDependencyCatalog, localModules map[string]struct{}) bool {
	top := strings.SplitN(module, ".", 2)[0]
	if top == "" || strings.HasPrefix(module, ".") {
		return true
	}
	if _, ok := pythonStdlibModuleSet[top]; ok {
		return true
	}
	if _, ok := localModules[top]; ok {
		return true
	}
	if pythonModuleOnDisk(root, filepath.Dir(file), top) {
		return true
	}
	if catalog.declares(top) {
		return true
	}
	for _, distribution := range pythonImportAliases[top] {
		if catalog.declares(distribution) {
			return true
		}
	}
	// Without any dependency manifest we cannot distinguish a hallucinated
	// import from an environment-provided package, so stay quiet.
	return !catalog.hasManifest
}

// pythonModuleOnDisk checks common locations for a module relative to the
// importing file, the repository root, and a conventional src/ layout.
func pythonModuleOnDisk(root string, dir string, module string) bool {
	bases := []string{filepath.Join(root, dir), root, filepath.Join(root, "src")}
	for _, base := range bases {
		if pathExists(filepath.Join(base, module+".py")) || pathExists(filepath.Join(base, module)) {
			return true
		}
	}
	return false
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
