package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

var (
	scriptImportPattern     = regexp.MustCompile(`(?m)(?:import\s+(?:[^'"]+?\s+from\s+)?|export\s+[^'"]+?\s+from\s+|require\(|import\()\s*['"]([^'"]+)['"]`)
	scriptDeadBranchPattern = regexp.MustCompile(`(?m)\b(if|while)\s*\(\s*(?:false|0)\s*\)`)
)

type scriptImportCatalog struct {
	hasManifest      bool
	deps             map[string]struct{}
	workspacePackage map[string]struct{}
}

func typeScriptAITargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	files, err := runnersupport.WalkFiles(target.Path, env.Config.Exclude, func(rel string) bool {
		lower := strings.ToLower(rel)
		return strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".tsx") || strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".jsx")
	})
	if err != nil {
		return nil
	}
	manifest, hasManifest := readPackageManifest(target.Path)
	catalog := scriptImportCatalog{
		hasManifest:      hasManifest,
		deps:             packageManifestDeps(manifest),
		workspacePackage: readWorkspacePackageNames(target.Path, env.Config.Exclude),
	}
	dominant := dominantScriptTestFramework(target.Path, files, manifest)
	input := scriptFileScanInput{
		catalog:    catalog,
		dominant:   dominant,
		errorStyle: dominantScriptErrorStyle(target.Path, files),
		naming:     dominantNamingConvention(target.Path, files, scriptDeclaredNames),
	}
	findings := make([]core.Finding, 0)
	for _, rel := range files {
		findings = append(findings, scriptFileAIQualityFindings(env, target.Path, rel, input)...)
	}
	return findings
}

type scriptFileScanInput struct {
	catalog    scriptImportCatalog
	dominant   string
	errorStyle string
	naming     string
}

func scriptFileAIQualityFindings(env support.Context, root string, rel string, input scriptFileScanInput) []core.Finding {
	abs := filepath.Join(root, rel)
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	checks := env.Config.Checks.QualityRules.AIChecks
	findings := make([]core.Finding, 0)
	if aiCheckEnabled(checks.HallucinatedImport) {
		findings = append(findings, scriptImportFindings(env, root, rel, source, input.catalog)...)
	}
	if aiCheckEnabled(checks.DeadCode) {
		findings = append(findings, scriptDeadCodeFindings(env, rel, source)...)
		findings = append(findings, scriptUnreachableFindings(env, rel, source)...)
		findings = append(findings, scriptUnusedFunctionFindings(env, rel, source)...)
	}
	if aiCheckEnabled(checks.ErrorStyleDrift) {
		findings = append(findings, scriptErrorStyleDriftFinding(env, rel, source, input.errorStyle)...)
	}
	if aiCheckEnabled(checks.NamingDrift) {
		findings = append(findings, namingDriftFinding(env, rel, source, input.naming, scriptDeclaredNames)...)
	}
	if isScriptTestFile(rel) {
		findings = append(findings, scriptOverMockedTestFinding(env, rel, source)...)
		findings = append(findings, scriptIdiomDriftFinding(env, rel, source, input.dominant)...)
	}
	return findings
}

func scriptImportFindings(env support.Context, root string, file string, source string, catalog scriptImportCatalog) []core.Finding {
	matches := scriptImportPattern.FindAllStringSubmatchIndex(source, -1)
	findings := make([]core.Finding, 0)
	for _, match := range matches {
		specifier := source[match[2]:match[3]]
		if scriptImportResolvable(root, file, specifier, catalog) {
			continue
		}
		line := 1 + strings.Count(source[:match[0]], "\n")
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.ai.hallucinated-import",
			Level:   "warn",
			Path:    file,
			Line:    line,
			Column:  1,
			Message: fmt.Sprintf("import %q does not resolve against package manifests, workspace packages, or local files", specifier),
		}))
	}
	return findings
}

func scriptImportResolvable(root string, file string, specifier string, catalog scriptImportCatalog) bool {
	switch {
	case specifier == "":
		return true
	case strings.HasPrefix(specifier, "."):
		return resolveRelativeScriptImport(root, filepath.Dir(file), specifier)
	case strings.HasPrefix(specifier, "/"), strings.HasPrefix(specifier, "@/"), strings.HasPrefix(specifier, "~/"), strings.HasPrefix(specifier, "#/"):
		return true
	}
	rootPackage := packageRoot(specifier)
	if isNodeBuiltinPackage(rootPackage) {
		return true
	}
	if _, ok := catalog.workspacePackage[rootPackage]; ok {
		return true
	}
	if _, ok := catalog.deps[rootPackage]; ok {
		return true
	}
	return !catalog.hasManifest
}

func resolveRelativeScriptImport(root string, dir string, specifier string) bool {
	base := filepath.Join(root, dir, filepath.FromSlash(specifier))
	candidates := []string{
		base, base + ".ts", base + ".tsx", base + ".js", base + ".jsx",
		base + ".mts", base + ".cts", base + ".mjs", base + ".cjs",
		filepath.Join(base, "index.ts"), filepath.Join(base, "index.tsx"),
		filepath.Join(base, "index.js"), filepath.Join(base, "index.jsx"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func scriptDeadCodeFindings(env support.Context, file string, source string) []core.Finding {
	lines := regexLineMatches(scriptDeadBranchPattern, source)
	findings := make([]core.Finding, 0, len(lines))
	for _, line := range lines {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.ai.dead-code",
			Level:   "warn",
			Path:    file,
			Line:    line,
			Column:  1,
			Message: "constant-condition branch leaves unreachable placeholder logic in the code path",
		}))
	}
	return findings
}

func scriptOverMockedTestFinding(env support.Context, file string, source string) []core.Finding {
	mockMarkers := []string{"jest.mock(", "vi.mock(", "sinon.stub(", "mockResolvedValue(", "mockReturnValue(", "mockImplementation("}
	assertMarkers := []string{"expect(", "assert.", "should.", "toEqual(", "toStrictEqual(", "toMatchObject("}
	mockCount := countMarkers(source, mockMarkers)
	assertCount := countMarkers(source, assertMarkers)
	if mockCount < 2 || assertCount > 1 {
		return nil
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "quality.ai.over-mocked-test",
		Level:   "warn",
		Path:    file,
		Line:    firstLineContaining(source, mockMarkers),
		Column:  1,
		Message: "test relies mostly on mocked collaborators with very little direct behavior assertion",
	})}
}
