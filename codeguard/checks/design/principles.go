package design

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func principleFindings(relativePath string, file *ast.File, rules designRules) []core.Finding {
	var findings []core.Finding

	if finding := packageNameFinding(relativePath, file.Name.Name, rules); finding != nil {
		findings = append(findings, *finding)
	}
	findings = append(findings, declCountFinding(relativePath, file, rules)...)
	findings = append(findings, methodCountFindings(relativePath, file, rules)...)
	findings = append(findings, interfaceFindings(relativePath, file, rules)...)

	return findings
}

func declCountFinding(relativePath string, file *ast.File, rules designRules) []core.Finding {
	if rules.maxDeclsPerFile == 0 || len(file.Decls) <= rules.maxDeclsPerFile {
		return nil
	}
	return []core.Finding{{
		Path:     relativePath,
		Message:  fmt.Sprintf("file has %d top-level declarations; limit is %d for separation of concerns", len(file.Decls), rules.maxDeclsPerFile),
		Severity: core.SeverityWarn,
	}}
}

func methodCountFindings(relativePath string, file *ast.File, rules designRules) []core.Finding {
	if rules.maxMethodsPerType == 0 {
		return nil
	}

	var findings []core.Finding
	for typeName, count := range methodsPerType(file) {
		if count <= rules.maxMethodsPerType {
			continue
		}
		findings = append(findings, core.Finding{
			Path:     relativePath,
			Message:  fmt.Sprintf("type %s has %d methods; limit is %d for single-responsibility", typeName, count, rules.maxMethodsPerType),
			Severity: core.SeverityWarn,
		})
	}
	return findings
}

func interfaceFindings(relativePath string, file *ast.File, rules designRules) []core.Finding {
	if rules.maxInterfaceMethods == 0 {
		return nil
	}

	var findings []core.Finding
	for _, decl := range file.Decls {
		findings = append(findings, interfaceDeclFindings(relativePath, decl, rules.maxInterfaceMethods)...)
	}
	return findings
}

func interfaceDeclFindings(relativePath string, decl ast.Decl, maxMethods int) []core.Finding {
	gen, ok := decl.(*ast.GenDecl)
	if !ok || gen.Tok != token.TYPE {
		return nil
	}

	var findings []core.Finding
	for _, spec := range gen.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		iface, ok := typeSpec.Type.(*ast.InterfaceType)
		if !ok {
			continue
		}
		methods := interfaceMethodCount(iface)
		if methods <= maxMethods {
			continue
		}
		findings = append(findings, core.Finding{
			Path:     relativePath,
			Message:  fmt.Sprintf("interface %s has %d methods; limit is %d for interface segregation", typeSpec.Name.Name, methods, maxMethods),
			Severity: core.SeverityWarn,
		})
	}
	return findings
}

func packageNameFinding(relativePath string, packageName string, rules designRules) *core.Finding {
	name := strings.ToLower(strings.TrimSpace(packageName))
	for _, forbidden := range rules.forbiddenPackageNames {
		if name == forbidden {
			return &core.Finding{
				Path:     relativePath,
				Message:  fmt.Sprintf("package name %s is too generic and weakens clean-code boundaries", packageName),
				Severity: core.SeverityWarn,
			}
		}
	}
	return nil
}

func methodsPerType(file *ast.File) map[string]int {
	counts := make(map[string]int)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		typeName := receiverTypeName(fn.Recv.List[0].Type)
		if typeName == "" {
			continue
		}
		counts[typeName]++
	}
	return counts
}

func receiverTypeName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		if ident, ok := value.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

func interfaceMethodCount(iface *ast.InterfaceType) int {
	if iface.Methods == nil {
		return 0
	}
	total := 0
	for _, field := range iface.Methods.List {
		if len(field.Names) == 0 {
			total++
			continue
		}
		total += len(field.Names)
	}
	return total
}
