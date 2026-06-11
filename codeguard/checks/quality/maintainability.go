package quality

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func maintainabilityFindings(path string, source []byte, file *ast.File, fset *token.FileSet, rules qualityRules) []core.Finding {
	var findings []core.Finding

	relativePath := filepath.ToSlash(path)
	findings = append(findings, fileLineFinding(relativePath, source, rules)...)
	findings = append(findings, functionFindings(relativePath, file, fset, rules)...)
	return findings
}

func fileLineFinding(path string, source []byte, rules qualityRules) []core.Finding {
	lineCount := countLines(source)
	if rules.maxFileLines == 0 || lineCount <= rules.maxFileLines {
		return nil
	}
	return []core.Finding{{
		Path:     path,
		Message:  fmt.Sprintf("file has %d lines; limit is %d", lineCount, rules.maxFileLines),
		Severity: core.SeverityWarn,
	}}
}

func countLines(source []byte) int {
	if len(source) == 0 {
		return 0
	}
	return bytes.Count(source, []byte{'\n'}) + 1
}

func functionFindings(path string, file *ast.File, fset *token.FileSet, rules qualityRules) []core.Finding {
	var findings []core.Finding
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		findings = append(findings, singleFunctionFindings(path, fn, fset, rules)...)
	}
	return findings
}

func singleFunctionFindings(path string, fn *ast.FuncDecl, fset *token.FileSet, rules qualityRules) []core.Finding {
	var findings []core.Finding
	findings = append(findings, functionLengthFinding(path, fn, fset, rules)...)
	findings = append(findings, functionParameterFinding(path, fn, rules)...)
	findings = append(findings, functionComplexityFinding(path, fn, rules)...)
	return findings
}

func functionLengthFinding(path string, fn *ast.FuncDecl, fset *token.FileSet, rules qualityRules) []core.Finding {
	if rules.maxFunctionLines == 0 {
		return nil
	}
	start := fset.Position(fn.Pos()).Line
	end := fset.Position(fn.End()).Line
	length := end - start + 1
	if length <= rules.maxFunctionLines {
		return nil
	}
	return []core.Finding{{
		Path:     path,
		Message:  fmt.Sprintf("function %s spans %d lines; limit is %d", fn.Name.Name, length, rules.maxFunctionLines),
		Severity: core.SeverityWarn,
	}}
}

func functionParameterFinding(path string, fn *ast.FuncDecl, rules qualityRules) []core.Finding {
	if rules.maxParameters == 0 {
		return nil
	}
	params := parameterCount(fn)
	if params <= rules.maxParameters {
		return nil
	}
	return []core.Finding{{
		Path:     path,
		Message:  fmt.Sprintf("function %s has %d parameters; limit is %d", fn.Name.Name, params, rules.maxParameters),
		Severity: core.SeverityWarn,
	}}
}

func functionComplexityFinding(path string, fn *ast.FuncDecl, rules qualityRules) []core.Finding {
	if rules.maxCyclomaticComplexity == 0 {
		return nil
	}
	complexity := cyclomaticComplexity(fn)
	if complexity <= rules.maxCyclomaticComplexity {
		return nil
	}
	return []core.Finding{{
		Path:     path,
		Message:  fmt.Sprintf("function %s has cyclomatic complexity %d; limit is %d", fn.Name.Name, complexity, rules.maxCyclomaticComplexity),
		Severity: core.SeverityWarn,
	}}
}

func parameterCount(fn *ast.FuncDecl) int {
	if fn.Type == nil || fn.Type.Params == nil {
		return 0
	}
	total := 0
	for _, field := range fn.Type.Params.List {
		if len(field.Names) == 0 {
			total++
			continue
		}
		total += len(field.Names)
	}
	return total
}

func cyclomaticComplexity(fn *ast.FuncDecl) int {
	complexity := 1
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			if node.Op == token.LAND || node.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return complexity
}
