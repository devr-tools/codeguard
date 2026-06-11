package runner

import (
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func (sc scanContext) runQuality(_ context.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range sc.cfg.Targets {
		findings = append(findings, scanTargetFiles(sc, target, "quality", func(rel string) bool {
			return strings.HasSuffix(rel, ".go")
		}, func(file string, data []byte) []core.Finding {
			return qualityFindingsForFile(sc, file, data)
		})...)
	}
	return finalizeSection(sc, "quality", "Code Quality", findings)
}

func qualityFindingsForFile(sc scanContext, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	lineCount := countLines(data)
	if lineCount > sc.cfg.Checks.QualityRules.MaxFileLines {
		findings = append(findings, newFinding(sc, findingInput{
			ruleID:  "quality.max-file-lines",
			level:   "warn",
			path:    file,
			line:    lineCount,
			column:  1,
			message: fmt.Sprintf("file has %d lines; max is %d", lineCount, sc.cfg.Checks.QualityRules.MaxFileLines),
		}))
	}

	formatted, err := format.Source(data)
	if err != nil {
		return append(findings, newFinding(sc, findingInput{
			ruleID:  "quality.parse-error",
			level:   "fail",
			path:    file,
			line:    1,
			column:  1,
			message: fmt.Sprintf("Go parse error: %v", err),
		}))
	}
	if string(formatted) != string(data) {
		findings = append(findings, newFinding(sc, findingInput{
			ruleID:  "quality.gofmt",
			level:   "fail",
			path:    file,
			line:    1,
			column:  1,
			message: "file is not gofmt-formatted",
		}))
	}

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, data, parser.ParseComments)
	if err != nil {
		return append(findings, newFinding(sc, findingInput{
			ruleID:  "quality.parse-error",
			level:   "fail",
			path:    file,
			line:    1,
			column:  1,
			message: fmt.Sprintf("Go parse error: %v", err),
		}))
	}
	if len(parsed.Decls) > sc.cfg.Checks.DesignRules.MaxDeclsPerFile {
		findings = append(findings, newFinding(sc, findingInput{
			ruleID:  "design.max-decls-per-file",
			level:   "warn",
			path:    file,
			line:    1,
			column:  1,
			message: fmt.Sprintf("file has %d declarations; max is %d", len(parsed.Decls), sc.cfg.Checks.DesignRules.MaxDeclsPerFile),
		}))
	}
	findings = append(findings, qualityImportFindings(sc, file, fset, parsed)...)
	findings = append(findings, qualityFunctionFindings(sc, file, fset, parsed)...)
	return findings
}

func qualityImportFindings(sc scanContext, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, imp := range parsed.Imports {
		pathValue := strings.Trim(imp.Path.Value, `"`)
		if strings.Contains(pathValue, "/internal/") && !isInternalOrCmdFile(file) {
			pos := fset.Position(imp.Pos())
			findings = append(findings, newFinding(sc, findingInput{
				ruleID:  "quality.dependency-direction",
				level:   "warn",
				path:    file,
				line:    pos.Line,
				column:  pos.Column,
				message: "non-CLI package imports internal implementation detail",
			}))
		}
	}
	return findings
}

func qualityFunctionFindings(sc scanContext, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	ast.Inspect(parsed, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		if end-start+1 > sc.cfg.Checks.QualityRules.MaxFunctionLines {
			findings = append(findings, newFinding(sc, findingInput{
				ruleID:  "quality.max-function-lines",
				level:   "warn",
				path:    file,
				line:    start,
				column:  1,
				message: fmt.Sprintf("function %s has %d lines; max is %d", fn.Name.Name, end-start+1, sc.cfg.Checks.QualityRules.MaxFunctionLines),
			}))
		}
		if params := countFuncParams(fn); params > sc.cfg.Checks.QualityRules.MaxParameters {
			findings = append(findings, newFinding(sc, findingInput{
				ruleID:  "quality.max-parameters",
				level:   "warn",
				path:    file,
				line:    start,
				column:  1,
				message: fmt.Sprintf("function %s has %d parameters; max is %d", fn.Name.Name, params, sc.cfg.Checks.QualityRules.MaxParameters),
			}))
		}
		if complexity := cyclomaticComplexity(fn.Body); complexity > sc.cfg.Checks.QualityRules.MaxCyclomaticComplexity {
			findings = append(findings, newFinding(sc, findingInput{
				ruleID:  "quality.cyclomatic-complexity",
				level:   "warn",
				path:    file,
				line:    start,
				column:  1,
				message: fmt.Sprintf("function %s has cyclomatic complexity %d; max is %d", fn.Name.Name, complexity, sc.cfg.Checks.QualityRules.MaxCyclomaticComplexity),
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

func (sc scanContext) runDesign(_ context.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range sc.cfg.Targets {
		findings = append(findings, scanTargetFiles(sc, target, "design", func(rel string) bool {
			return strings.HasSuffix(rel, ".go")
		}, func(file string, data []byte) []core.Finding {
			return designFindingsForFile(sc, file, data)
		})...)
	}
	return finalizeSection(sc, "design", "Design Patterns", findings)
}

func designFindingsForFile(sc scanContext, file string, data []byte) []core.Finding {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, data, parser.ParseComments)
	if err != nil {
		return nil
	}

	findings := make([]core.Finding, 0)
	findings = append(findings, forbiddenPackageFindings(sc, file, parsed.Name.Name)...)
	methodCounts, interfaceFindings := designTypeFindings(sc, file, fset, parsed)
	findings = append(findings, interfaceFindings...)
	findings = append(findings, designMethodFindings(sc, file, methodCounts)...)
	findings = append(findings, designImportFindings(sc, file, fset, parsed)...)
	return findings
}

func forbiddenPackageFindings(sc scanContext, file string, pkgName string) []core.Finding {
	for _, forbidden := range sc.cfg.Checks.DesignRules.ForbiddenPackageNames {
		if pkgName == forbidden {
			return []core.Finding{newFinding(sc, findingInput{
				ruleID:  "design.generic-package-name",
				level:   "warn",
				path:    file,
				line:    1,
				column:  1,
				message: fmt.Sprintf("package name %q is too generic", pkgName),
			})}
		}
	}
	return nil
}

func designTypeFindings(sc scanContext, file string, fset *token.FileSet, parsed *ast.File) (map[string]int, []core.Finding) {
	methodCounts := map[string]int{}
	findings := make([]core.Finding, 0)
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && len(fn.Recv.List) > 0 {
			methodCounts[typeName(fn.Recv.List[0].Type)]++
		}
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			iface, ok := ts.Type.(*ast.InterfaceType)
			if !ok || iface.Methods == nil || len(iface.Methods.List) <= sc.cfg.Checks.DesignRules.MaxInterfaceMethods {
				continue
			}
			pos := fset.Position(ts.Pos())
			findings = append(findings, newFinding(sc, findingInput{
				ruleID:  "design.max-interface-methods",
				level:   "warn",
				path:    file,
				line:    pos.Line,
				column:  pos.Column,
				message: fmt.Sprintf("interface %s has %d methods; max is %d", ts.Name.Name, len(iface.Methods.List), sc.cfg.Checks.DesignRules.MaxInterfaceMethods),
			}))
		}
	}
	return methodCounts, findings
}

func designMethodFindings(sc scanContext, file string, methodCounts map[string]int) []core.Finding {
	findings := make([]core.Finding, 0)
	for recv, count := range methodCounts {
		if count > sc.cfg.Checks.DesignRules.MaxMethodsPerType {
			findings = append(findings, newFinding(sc, findingInput{
				ruleID:  "design.max-methods-per-type",
				level:   "warn",
				path:    file,
				line:    1,
				column:  1,
				message: fmt.Sprintf("type %s has %d methods; max is %d", recv, count, sc.cfg.Checks.DesignRules.MaxMethodsPerType),
			}))
		}
	}
	return findings
}

func designImportFindings(sc scanContext, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	normalized := filepath.ToSlash(file)
	for _, imp := range parsed.Imports {
		pathValue := strings.Trim(imp.Path.Value, `"`)
		pos := fset.Position(imp.Pos())
		if *sc.cfg.Checks.DesignRules.RequireCmdThroughInternalCLI &&
			isCmdFile(normalized) &&
			strings.Contains(pathValue, "/codeguard/") &&
			!strings.Contains(pathValue, "/internal/") {
			findings = append(findings, newFinding(sc, findingInput{
				ruleID:  "design.cmd-through-internal-cli",
				level:   "fail",
				path:    file,
				line:    pos.Line,
				column:  pos.Column,
				message: "cmd package imports reusable service package directly",
			}))
		}
		if *sc.cfg.Checks.DesignRules.ForbidServiceImportInternal && isServicePackageFile(normalized) && strings.Contains(pathValue, "/internal/") {
			findings = append(findings, newFinding(sc, findingInput{
				ruleID:  "design.service-import-internal",
				level:   "fail",
				path:    file,
				line:    pos.Line,
				column:  pos.Column,
				message: "service package imports internal package",
			}))
		}
		if *sc.cfg.Checks.DesignRules.ForbidServiceImportCmd && isServicePackageFile(normalized) && strings.Contains(pathValue, "/cmd/") {
			findings = append(findings, newFinding(sc, findingInput{
				ruleID:  "design.service-import-cmd",
				level:   "fail",
				path:    file,
				line:    pos.Line,
				column:  pos.Column,
				message: "service package imports cmd package",
			}))
		}
	}
	return findings
}
