package design

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func goTargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	// Resolved once per target rather than per file: it reads go.mod and lists
	// pkg/ on the first call.
	facade := support.SDKFacadeForTarget(env, target)
	// The facade comes from go.mod and the pkg/ layout, not the scanned file,
	// so its fingerprint is part of every cached entry's key.
	deps := support.FileScanDependencies{Key: facade.CacheKey()}
	return support.ScanTargetFilesWithDependencies(env, target, "design", func(rel string) bool {
		return strings.HasSuffix(rel, ".go")
	}, deps, func(file string, data []byte) []core.Finding {
		return goFindingsForFile(env, facade, file, data)
	})
}

func goFindingsForFile(env support.Context, facade support.SDKFacade, file string, data []byte) []core.Finding {
	fset, parsed, err := support.ParseGoSource(env, file, data)
	if err != nil {
		return nil
	}

	findings := make([]core.Finding, 0) //nolint:prealloc // count not known up front; each rule appends a variable number
	findings = append(findings, forbiddenPackageFindings(env, file, parsed.Name.Name)...)
	findings = append(findings, declFindings(env, file, parsed)...)
	methodCounts, interfaceFindings := typeFindings(env, file, fset, parsed)
	findings = append(findings, interfaceFindings...)
	findings = append(findings, methodFindings(env, file, methodCounts)...)
	findings = append(findings, importFindings(env, facade, file, fset, parsed)...)
	return findings
}

func declFindings(env support.Context, file string, parsed *ast.File) []core.Finding {
	if len(parsed.Decls) <= env.Config.Checks.DesignRules.MaxDeclsPerFile {
		return nil
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "design.max-decls-per-file",
		Level:   "warn",
		Path:    file,
		Line:    1,
		Column:  1,
		Message: fmt.Sprintf("file has %d declarations; max is %d", len(parsed.Decls), env.Config.Checks.DesignRules.MaxDeclsPerFile),
	})}
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

func importFindings(env support.Context, facade support.SDKFacade, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0) //nolint:prealloc // count not known up front; each import appends a variable number
	normalized := filepath.ToSlash(file)
	for _, imp := range parsed.Imports {
		pathValue := strings.Trim(imp.Path.Value, `"`)
		pos := fset.Position(imp.Pos())
		findings = append(findings, cmdImportFindings(env, facade, file, normalized, pathValue, pos)...)
		findings = append(findings, publicPackageImportFindings(env, facade, file, normalized, pathValue, pos)...)
	}
	return findings
}

// cmdImportFindings reports a cmd/ package reaching for the repository's own
// public facade instead of the internal CLI layer.
//
// The facade used to be the literal "/pkg/codeguard", so this rule could only
// ever fire inside codeguard's own tree - it shipped as a fail-level default
// that was a guaranteed no-op for every other user. It now resolves the
// facade from the scanned repository, module-qualified when a go.mod is
// readable so an unrelated third-party package of the same name cannot match.
func cmdImportFindings(env support.Context, facade support.SDKFacade, file string, normalized string, pathValue string, pos token.Position) []core.Finding {
	if !*env.Config.Checks.DesignRules.RequireCmdThroughInternalCLI {
		return nil
	}
	if !env.IsCmdFile(normalized) || !facade.MatchesImport(pathValue) || strings.Contains(pathValue, "/internal/") {
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

func publicPackageImportFindings(env support.Context, facade support.SDKFacade, file string, normalized string, pathValue string, pos token.Position) []core.Finding {
	if !env.IsPublicPackageFile(normalized) {
		return nil
	}

	findings := make([]core.Finding, 0, 2)
	// The facade exemption applies to the internal-import rule only. A public
	// package wrapping internal/ is the whole point of a facade, and within a
	// single Go module it is legal, idiomatic code - so the exemption has to be
	// generous or the rule reports correct code at fail level. Importing cmd/
	// is a layering inversion under any layout, facade included, so that rule
	// keeps no exemption.
	if *env.Config.Checks.DesignRules.ForbidServiceImportInternal && !facade.MatchesFile(normalized) && strings.Contains(pathValue, "/internal/") {
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
