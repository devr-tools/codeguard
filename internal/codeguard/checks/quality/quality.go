package quality

import (
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Run(_ context.Context, env support.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
		findings = append(findings, env.ScanTargetFiles(target, "quality", func(rel string) bool {
			return strings.HasSuffix(rel, ".go")
		}, func(file string, data []byte) []core.Finding {
			return findingsForFile(env, file, data)
		})...)
	}
	return env.FinalizeSection("quality", "Code Quality", findings)
}

func findingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	lineCount := env.CountLines(data)
	if lineCount > env.Config.Checks.QualityRules.MaxFileLines {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.max-file-lines",
			Level:   "warn",
			Path:    file,
			Line:    lineCount,
			Column:  1,
			Message: fmt.Sprintf("file has %d lines; max is %d", lineCount, env.Config.Checks.QualityRules.MaxFileLines),
		}))
	}

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
	findings = append(findings, functionFindings(env, file, fset, parsed)...)
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
	return !env.IsSDKFacadeFile(file)
}

func functionFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	ast.Inspect(parsed, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		if end-start+1 > env.Config.Checks.QualityRules.MaxFunctionLines {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "quality.max-function-lines",
				Level:   "warn",
				Path:    file,
				Line:    start,
				Column:  1,
				Message: fmt.Sprintf("function %s has %d lines; max is %d", fn.Name.Name, end-start+1, env.Config.Checks.QualityRules.MaxFunctionLines),
			}))
		}
		if params := countFuncParams(fn); params > env.Config.Checks.QualityRules.MaxParameters {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "quality.max-parameters",
				Level:   "warn",
				Path:    file,
				Line:    start,
				Column:  1,
				Message: fmt.Sprintf("function %s has %d parameters; max is %d", fn.Name.Name, params, env.Config.Checks.QualityRules.MaxParameters),
			}))
		}
		if complexity := env.CyclomaticComplexity(fn.Body); complexity > env.Config.Checks.QualityRules.MaxCyclomaticComplexity {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "quality.cyclomatic-complexity",
				Level:   "warn",
				Path:    file,
				Line:    start,
				Column:  1,
				Message: fmt.Sprintf("function %s has cyclomatic complexity %d; max is %d", fn.Name.Name, complexity, env.Config.Checks.QualityRules.MaxCyclomaticComplexity),
			}))
		}
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
