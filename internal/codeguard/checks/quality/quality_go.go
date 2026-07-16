package quality

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func goFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
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
	if len(parsed.Decls) > env.Config.Checks.DesignRules.MaxDeclsPerFile {
		findings = append(findings, warnFinding(env, "design.max-decls-per-file", file, 1, 1,
			fmt.Sprintf("file has %d declarations; max is %d", len(parsed.Decls), env.Config.Checks.DesignRules.MaxDeclsPerFile)))
	}
	findings = append(findings, importFindings(env, file, fset, parsed)...)
	findings = append(findings, goFunctionFindings(env, file, fset, parsed)...)
	findings = append(findings, goAIQualityFindings(env, file, fset, parsed, data)...)
	return append(fileLengthFindingWithSignals(env, file, data, findings), findings...)
}

func importFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, imp := range parsed.Imports {
		pathValue := strings.Trim(imp.Path.Value, `"`)
		if strings.Contains(pathValue, "/internal/") && allowsInternalImport(env, file) {
			pos := fset.Position(imp.Pos())
			findings = append(findings, warnFinding(env, "quality.dependency-direction", file, pos.Line, pos.Column,
				"non-CLI package imports internal implementation detail"))
		}
	}
	return findings
}

func allowsInternalImport(env support.Context, file string) bool {
	if env.IsInternalOrCmdFile(file) {
		return false
	}
	if strings.HasPrefix(filepath.ToSlash(file), "tests/") {
		return false
	}
	return !env.IsSDKFacadeFile(file)
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
