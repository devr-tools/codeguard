package quality

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func goAITargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	files := aiTargetSourceFiles(env, target, ".go")
	if len(files) == 0 {
		return nil
	}
	resolver := newGoModuleResolver(target.Path)
	profile := goRepoStyleProfile(env, target, files)
	packageFiles := map[string][]goParsedFile{}
	findings := make([]core.Finding, 0)
	for _, rel := range files {
		fileFindings, parsedFile := goFileAIQualityFindings(env, target, rel, goFileScanInput{
			metadata:   resolver.metadataForFile(rel),
			dominant:   profile.testFramework,
			errorStyle: profile.errorStyle,
			naming:     profile.naming,
		})
		findings = append(findings, fileFindings...)
		if parsedFile != nil {
			dir := filepath.Dir(rel)
			packageFiles[dir] = append(packageFiles[dir], *parsedFile)
		}
	}
	if aiCheckEnabled(env.Config.Checks.QualityRules.AIChecks.DeadCode) {
		for _, parsedFiles := range packageFiles {
			findings = append(findings, goUnusedPrivateFunctionFindings(env, parsedFiles)...)
		}
	}
	return findings
}

type goFileScanInput struct {
	metadata   goModuleMetadata
	dominant   string
	errorStyle string
	naming     string
}

type goRepoStyle struct {
	testFramework string
	errorStyle    string
	naming        string
}

// goRepoStyleProfile computes the three repository-dominant style signals
// (test framework, error style, naming convention) in a single pass over the
// corpus instead of one read of every file per signal. Each signal is an
// independent per-file sum, so folding them into one loop is
// behavior-identical to the previous three passes.
func goRepoStyleProfile(env support.Context, target core.TargetConfig, files []string) goRepoStyle {
	frameworkCounts := map[string]int{}
	styleTotals := map[string]int{}
	namingTotals := map[string]int{}
	for _, rel := range files {
		data, err := readAITargetFile(env, target, rel)
		if err != nil {
			continue
		}
		source := string(data)
		if framework := goTestFramework(source); framework != "" {
			frameworkCounts[framework]++
		}
		for style, count := range goErrorStyleCounts(source) {
			styleTotals[style] += count
		}
		for convention, count := range namingCounts(source, goDeclaredNames) {
			namingTotals[convention] += count
		}
	}
	return goRepoStyle{
		testFramework: dominantFrameworkFromCounts(frameworkCounts),
		errorStyle:    dominantStyleFromTotals(styleTotals),
		naming:        dominantStyleFromTotals(namingTotals),
	}
}

func goFileAIQualityFindings(env support.Context, target core.TargetConfig, rel string, input goFileScanInput) ([]core.Finding, *goParsedFile) {
	data, err := readAITargetFile(env, target, rel)
	if err != nil {
		return nil, nil
	}
	source := string(data)
	checks := env.Config.Checks.QualityRules.AIChecks
	findings := make([]core.Finding, 0)
	var parsedFile *goParsedFile
	if fset, parsed, err := support.ParseGoSource(env, rel, data); err == nil {
		parsedFile = &goParsedFile{rel: rel, fset: fset, parsed: parsed}
		if aiCheckEnabled(checks.HallucinatedImport) {
			findings = append(findings, goHallucinatedImportFindings(env, rel, fset, parsed, input.metadata)...)
		}
		if aiCheckEnabled(checks.DeadCode) {
			findings = append(findings, goDeadCodeFindings(env, rel, fset, parsed)...)
			findings = append(findings, goUnreachableCodeFindings(env, rel, fset, parsed)...)
		}
	}
	if aiCheckEnabled(checks.ErrorStyleDrift) {
		findings = append(findings, goErrorStyleDriftFinding(env, rel, source, input.errorStyle)...)
	}
	if aiCheckEnabled(checks.NamingDrift) {
		findings = append(findings, namingDriftFinding(env, rel, source, input.naming, goDeclaredNames)...)
	}
	if strings.HasSuffix(rel, "_test.go") {
		findings = append(findings, goOverMockedTestFinding(env, rel, source)...)
		findings = append(findings, goIdiomDriftFinding(env, rel, source, input.dominant)...)
	}
	return findings, parsedFile
}

func goHallucinatedImportFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File, metadata goModuleMetadata) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, imp := range parsed.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if goImportResolvable(importPath, metadata) {
			continue
		}
		pos := fset.Position(imp.Pos())
		findings = append(findings, warnFinding(env, "quality.ai.hallucinated-import", file, pos.Line, pos.Column,
			fmt.Sprintf("import %q does not resolve against go.mod or the local module path", importPath)))
	}
	return findings
}

func goImportResolvable(importPath string, metadata goModuleMetadata) bool {
	return metadata.resolvesImport(importPath)
}

func goDeadCodeFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	ast.Inspect(parsed, func(node ast.Node) bool {
		ifStmt, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		ident, ok := ifStmt.Cond.(*ast.Ident)
		if !ok || ident.Name != "false" {
			return true
		}
		pos := fset.Position(ifStmt.Pos())
		findings = append(findings, warnFinding(env, "quality.ai.dead-code", file, pos.Line, pos.Column,
			"constant false branch leaves unreachable placeholder logic in the code path"))
		return true
	})
	return findings
}

func goOverMockedTestFinding(env support.Context, file string, source string) []core.Finding {
	mockMarkers := []string{"gomock.", "mock.", "EXPECT()", "NewMock", "On(", ".Return("}
	assertMarkers := []string{"assert.", "require.", "t.Fatalf(", "t.Errorf(", "t.Helper()", "cmp.Diff("}
	mockCount := countMarkers(source, mockMarkers)
	assertCount := countMarkers(source, assertMarkers)
	if mockCount < 4 || assertCount > 1 {
		return nil
	}
	return []core.Finding{warnFinding(env, "quality.ai.over-mocked-test", file, firstLineContaining(source, mockMarkers), 1,
		"test is dominated by mock setup and expectations with very little direct behavior assertion")}
}
