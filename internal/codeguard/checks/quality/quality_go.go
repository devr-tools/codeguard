package quality

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func goFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := fileLengthFinding(env, file, data)

	formatted, err := format.Source(data)
	if err != nil {
		return append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.parse-error",
			Level:   "fail",
			Path:    file,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("Go parse error: %v", err),
		}))
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

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, data, parser.ParseComments)
	if err != nil {
		return append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.parse-error",
			Level:   "fail",
			Path:    file,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("Go parse error: %v", err),
		}))
	}
	if len(parsed.Decls) > env.Config.Checks.DesignRules.MaxDeclsPerFile {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "design.max-decls-per-file",
			Level:   "warn",
			Path:    file,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("file has %d declarations; max is %d", len(parsed.Decls), env.Config.Checks.DesignRules.MaxDeclsPerFile),
		}))
	}
	findings = append(findings, importFindings(env, file, fset, parsed)...)
	findings = append(findings, goFunctionFindings(env, file, fset, parsed)...)
	return findings
}

func importFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, imp := range parsed.Imports {
		pathValue := strings.Trim(imp.Path.Value, `"`)
		if strings.Contains(pathValue, "/internal/") && allowsInternalImport(env, file) {
			pos := fset.Position(imp.Pos())
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "quality.dependency-direction",
				Level:   "warn",
				Path:    file,
				Line:    pos.Line,
				Column:  pos.Column,
				Message: "non-CLI package imports internal implementation detail",
			}))
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
