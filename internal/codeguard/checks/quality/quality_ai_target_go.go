package quality

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type goModuleMetadata struct {
	modulePath string
	required   []string
}

func goAITargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	files := aiTargetSourceFiles(env, target, ".go")
	if len(files) == 0 {
		return nil
	}
	metadata := readGoModuleMetadata(target.Path)
	dominant := dominantGoTestFramework(target.Path, files)
	errorStyle := dominantGoErrorStyle(target.Path, files)
	naming := dominantNamingConvention(target.Path, files, goDeclaredNames)
	packageFiles := map[string][]goParsedFile{}
	findings := make([]core.Finding, 0)
	for _, rel := range files {
		fileFindings, parsedFile := goFileAIQualityFindings(env, target.Path, rel, goFileScanInput{
			metadata:   metadata,
			dominant:   dominant,
			errorStyle: errorStyle,
			naming:     naming,
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

func goFileAIQualityFindings(env support.Context, root string, rel string, input goFileScanInput) ([]core.Finding, *goParsedFile) {
	abs := filepath.Join(root, rel)
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, nil
	}
	source := string(data)
	checks := env.Config.Checks.QualityRules.AIChecks
	findings := make([]core.Finding, 0)
	var parsedFile *goParsedFile
	fset := token.NewFileSet()
	if parsed, err := parser.ParseFile(fset, abs, data, 0); err == nil {
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

func readGoModuleMetadata(root string) goModuleMetadata {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return goModuleMetadata{}
	}
	metadata := goModuleMetadata{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "module":
			metadata.modulePath = fields[1]
		case "go", "replace", "exclude", "retract":
			continue
		case "require":
			if len(fields) >= 3 {
				metadata.required = append(metadata.required, fields[1])
			}
		default:
			if strings.HasPrefix(fields[1], "v") {
				metadata.required = append(metadata.required, fields[0])
			}
		}
	}
	return metadata
}

func goHallucinatedImportFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File, metadata goModuleMetadata) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, imp := range parsed.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if goImportResolvable(importPath, metadata) {
			continue
		}
		pos := fset.Position(imp.Pos())
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.ai.hallucinated-import",
			Level:   "warn",
			Path:    file,
			Line:    pos.Line,
			Column:  pos.Column,
			Message: fmt.Sprintf("import %q does not resolve against go.mod or the local module path", importPath),
		}))
	}
	return findings
}

func goImportResolvable(importPath string, metadata goModuleMetadata) bool {
	if importPath == "" {
		return true
	}
	if !strings.Contains(firstSegment(importPath), ".") {
		return true
	}
	if metadata.modulePath != "" && (importPath == metadata.modulePath || strings.HasPrefix(importPath, metadata.modulePath+"/")) {
		return true
	}
	for _, required := range metadata.required {
		if importPath == required || strings.HasPrefix(importPath, required+"/") {
			return true
		}
	}
	return false
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
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.ai.dead-code",
			Level:   "warn",
			Path:    file,
			Line:    pos.Line,
			Column:  pos.Column,
			Message: "constant false branch leaves unreachable placeholder logic in the code path",
		}))
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
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "quality.ai.over-mocked-test",
		Level:   "warn",
		Path:    file,
		Line:    firstLineContaining(source, mockMarkers),
		Column:  1,
		Message: "test is dominated by mock setup and expectations with very little direct behavior assertion",
	})}
}
