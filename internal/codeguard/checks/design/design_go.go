package design

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func goTargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	return env.ScanTargetFiles(target, "design", func(rel string) bool {
		return strings.HasSuffix(rel, ".go")
	}, func(file string, data []byte) []core.Finding {
		return goFindingsForFile(env, file, data)
	})
}

func goFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, data, parser.ParseComments)
	if err != nil {
		return nil
	}

	findings := make([]core.Finding, 0) //nolint:prealloc // count not known up front; each rule appends a variable number
	findings = append(findings, forbiddenPackageFindings(env, file, parsed.Name.Name)...)
	methodCounts, interfaceFindings := typeFindings(env, file, fset, parsed)
	findings = append(findings, interfaceFindings...)
	findings = append(findings, methodFindings(env, file, methodCounts)...)
	findings = append(findings, importFindings(env, file, fset, parsed)...)
	return findings
}

func forbiddenPackageFindings(env support.Context, file string, pkgName string) []core.Finding {
	for _, forbidden := range env.Config.Checks.DesignRules.ForbiddenPackageNames {
		if pkgName == forbidden {
			return []core.Finding{env.NewFinding(support.FindingInput{
				RuleID:  "design.generic-package-name",
				Level:   "warn",
				Path:    file,
				Line:    1,
				Column:  1,
				Message: fmt.Sprintf("package name %q is too generic", pkgName),
			})}
		}
	}
	return nil
}

func typeFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) (map[string]int, []core.Finding) {
	methodCounts := map[string]int{}
	findings := make([]core.Finding, 0)
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && len(fn.Recv.List) > 0 {
			methodCounts[env.TypeName(fn.Recv.List[0].Type)]++
		}
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		findings = append(findings, interfaceFindings(env, file, fset, gd)...)
	}
	return methodCounts, findings
}

func interfaceFindings(env support.Context, file string, fset *token.FileSet, gd *ast.GenDecl) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, spec := range gd.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		iface, ok := ts.Type.(*ast.InterfaceType)
		if !ok || iface.Methods == nil || len(iface.Methods.List) <= env.Config.Checks.DesignRules.MaxInterfaceMethods {
			continue
		}
		pos := fset.Position(ts.Pos())
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "design.max-interface-methods",
			Level:   "warn",
			Path:    file,
			Line:    pos.Line,
			Column:  pos.Column,
			Message: fmt.Sprintf("interface %s has %d methods; max is %d", ts.Name.Name, len(iface.Methods.List), env.Config.Checks.DesignRules.MaxInterfaceMethods),
		}))
	}
	return findings
}

func methodFindings(env support.Context, file string, methodCounts map[string]int) []core.Finding {
	findings := make([]core.Finding, 0)
	for recv, count := range methodCounts {
		if count > env.Config.Checks.DesignRules.MaxMethodsPerType {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.max-methods-per-type",
				Level:   "warn",
				Path:    file,
				Line:    1,
				Column:  1,
				Message: fmt.Sprintf("type %s has %d methods; max is %d", recv, count, env.Config.Checks.DesignRules.MaxMethodsPerType),
			}))
		}
	}
	return findings
}

func importFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0) //nolint:prealloc // count not known up front; each import appends a variable number
	normalized := filepath.ToSlash(file)
	for _, imp := range parsed.Imports {
		pathValue := strings.Trim(imp.Path.Value, `"`)
		pos := fset.Position(imp.Pos())
		findings = append(findings, cmdImportFindings(env, file, normalized, pathValue, pos)...)
		findings = append(findings, publicPackageImportFindings(env, file, normalized, pathValue, pos)...)
	}
	return findings
}

func cmdImportFindings(env support.Context, file string, normalized string, pathValue string, pos token.Position) []core.Finding {
	if !*env.Config.Checks.DesignRules.RequireCmdThroughInternalCLI {
		return nil
	}
	if !env.IsCmdFile(normalized) || !strings.Contains(pathValue, "/pkg/codeguard") || strings.Contains(pathValue, "/internal/") {
		return nil
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "design.cmd-through-internal-cli",
		Level:   "fail",
		Path:    file,
		Line:    pos.Line,
		Column:  pos.Column,
		Message: "cmd package imports reusable service package directly",
	})}
}

func publicPackageImportFindings(env support.Context, file string, normalized string, pathValue string, pos token.Position) []core.Finding {
	if !env.IsPublicPackageFile(normalized) || env.IsSDKFacadeFile(normalized) {
		return nil
	}

	findings := make([]core.Finding, 0, 2)
	if *env.Config.Checks.DesignRules.ForbidServiceImportInternal && strings.Contains(pathValue, "/internal/") {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "design.service-import-internal",
			Level:   "fail",
			Path:    file,
			Line:    pos.Line,
			Column:  pos.Column,
			Message: "service package imports internal package",
		}))
	}
	if *env.Config.Checks.DesignRules.ForbidServiceImportCmd && strings.Contains(pathValue, "/cmd/") {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "design.service-import-cmd",
			Level:   "fail",
			Path:    file,
			Line:    pos.Line,
			Column:  pos.Column,
			Message: "service package imports cmd package",
		}))
	}
	return findings
}
