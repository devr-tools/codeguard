package design

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func layerFindings(relativePath string, file *ast.File, rules designRules) []core.Finding {
	imports := importPaths(file)
	layer := classifyLayer(relativePath)
	switch layer {
	case layerCmd:
		return cmdLayerFindings(relativePath, imports, rules)
	case layerInternal:
		return internalLayerFindings(relativePath, imports, rules)
	case layerService:
		return serviceLayerFindings(relativePath, imports, rules)
	default:
		return nil
	}
}

func cmdLayerFindings(relativePath string, imports []string, rules designRules) []core.Finding {
	if !rules.requireCmdThroughInternalCLI {
		return nil
	}

	var findings []core.Finding
	for _, imp := range imports {
		if !isServiceImportBypassingCLI(imp) {
			continue
		}
		findings = append(findings, core.Finding{
			Path:     relativePath,
			Message:  fmt.Sprintf("cmd entrypoint imports service package %s instead of going through internal/cli", imp),
			Severity: core.SeverityError,
		})
	}
	return findings
}

func internalLayerFindings(relativePath string, imports []string, rules designRules) []core.Finding {
	if !rules.forbidInternalImportCmd {
		return nil
	}

	var findings []core.Finding
	for _, imp := range imports {
		if !strings.Contains(imp, "/cmd/") {
			continue
		}
		findings = append(findings, core.Finding{
			Path:     relativePath,
			Message:  fmt.Sprintf("internal package imports command package %s", imp),
			Severity: core.SeverityError,
		})
	}
	return findings
}

func serviceLayerFindings(relativePath string, imports []string, rules designRules) []core.Finding {
	var findings []core.Finding
	for _, imp := range imports {
		findings = append(findings, serviceImportFindings(relativePath, imp, rules)...)
	}
	return findings
}

func serviceImportFindings(relativePath string, imp string, rules designRules) []core.Finding {
	var findings []core.Finding
	if rules.forbidServiceImportInternal && strings.Contains(imp, "/internal/") {
		findings = append(findings, core.Finding{
			Path:     relativePath,
			Message:  fmt.Sprintf("reusable service package imports internal package %s", imp),
			Severity: core.SeverityError,
		})
	}
	if rules.forbidServiceImportCmd && strings.Contains(imp, "/cmd/") {
		findings = append(findings, core.Finding{
			Path:     relativePath,
			Message:  fmt.Sprintf("reusable service package imports command package %s", imp),
			Severity: core.SeverityError,
		})
	}
	return findings
}

func isServiceImportBypassingCLI(imp string) bool {
	return strings.Contains(imp, "/codeguard/checks/") ||
		strings.HasSuffix(imp, "/codeguard") ||
		strings.Contains(imp, "/codeguard/config") ||
		strings.Contains(imp, "/codeguard/runner")
}

type layer string

const (
	layerOther    layer = "other"
	layerCmd      layer = "cmd"
	layerInternal layer = "internal"
	layerService  layer = "service"
)

func classifyLayer(relativePath string) layer {
	switch {
	case strings.HasPrefix(relativePath, "cmd/"):
		return layerCmd
	case strings.HasPrefix(relativePath, "internal/"):
		return layerInternal
	case strings.HasPrefix(relativePath, "codeguard/"), relativePath == "sdk.go":
		return layerService
	default:
		return layerOther
	}
}

func trimImportPath(value string) string {
	return strings.Trim(value, "\"")
}
