package quality

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func goFindingsForFileWithIndex(env support.Context, file string, data []byte, index *goPackageIndex, modules *goModuleResolver) []core.Finding {
	findings := make([]core.Finding, 0)

	formatted, err := format.Source(data)
	if err != nil {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.parse-error",
			Level:   "fail",
			Path:    file,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("Go parse error: %v", err),
		}))
		return append(fileLengthFindingWithSignals(env, file, data, findings), findings...)
	}
	if string(formatted) != string(data) {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.gofmt",
			Level:   "fail",
			Path:    file,
			Line:    1,
			Column:  1,
			Message: "file is not gofmt-formatted",
		}))
	}

	fset, parsed, err := support.ParseGoSource(env, file, data)
	if err != nil {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.parse-error",
			Level:   "fail",
			Path:    file,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("Go parse error: %v", err),
		}))
		return append(fileLengthFindingWithSignals(env, file, data, findings), findings...)
	}
	if index == nil {
		index = newGoPackageIndex()
		index.addFile(file, fset, parsed)
	}
	pkg := index.packageFor(file, parsed.Name.Name)
	findings = append(findings, importFindings(env, file, fset, parsed, modules)...)
	findings = append(findings, goFunctionFindings(env, file, fset, parsed)...)
	if localPrecisionEnabled(env) {
		findings = append(findings, goPrecisionFindings(env, file, fset, parsed, data, pkg)...)
		findings = append(findings, goStructuralSmellFindings(env, file, fset, parsed, data)...)
	}
	findings = append(findings, goAIQualityFindings(env, file, fset, parsed, data)...)
	return append(fileLengthFindingWithSignals(env, file, data, findings), findings...)
}

// importFindings reports imports of an internal package tree that Go's
// internal-visibility rule does not permit, i.e. the importing package is
// outside the directory the internal tree is scoped to.
//
// It deliberately does NOT report same-module internal imports. Importing
// "<module>/internal/x" from anywhere in that module - cmd/, pkg/, a public
// facade, an external test package - is legal and idiomatic Go, and codeguard
// measured 631 findings across four repositories of which every single one was
// such an import. See the rule's catalog entry for the remaining scope.
func importFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File, modules *goModuleResolver) []core.Finding {
	findings := make([]core.Finding, 0)
	if testSourceFile(env, file) {
		return findings
	}
	// The importing package path is resolved lazily: it takes the module
	// resolver's lock, and most files import no internal package at all.
	importer := ""
	resolved := false
	for _, imp := range parsed.Imports {
		pathValue := strings.Trim(imp.Path.Value, `"`)
		owner, ok := internalPackageOwner(pathValue)
		if !ok {
			continue
		}
		if !resolved {
			importer = goPackagePathForFile(modules, file)
			resolved = true
		}
		if importer == "" || goInternalImportVisible(importer, owner) {
			continue
		}
		pos := fset.Position(imp.Pos())
		findings = append(findings, warnFinding(env, "quality.dependency-direction", file, pos.Line, pos.Column,
			fmt.Sprintf("package %q imports internal tree %q, which is scoped to %q and its descendants", importer, pathValue, owner)))
	}
	return findings
}

func goFunctionFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	ast.Inspect(parsed, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		findings = append(findings, maintainabilityFindings(env, file, functionMetrics{
			Name:       fn.Name.Name,
			StartLine:  start,
			Length:     end - start + 1,
			Params:     countFuncParams(fn),
			Complexity: env.CyclomaticComplexity(fn.Body),
		})...)
		return true
	})
	return findings
}

func countFuncParams(fn *ast.FuncDecl) int {
	if fn.Type == nil || fn.Type.Params == nil {
		return 0
	}
	paramCount := 0
	for _, param := range fn.Type.Params.List {
		if len(param.Names) == 0 {
			paramCount++
			continue
		}
		paramCount += len(param.Names)
	}
	return paramCount
}
