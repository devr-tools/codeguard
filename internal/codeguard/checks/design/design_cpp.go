package design

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	cppTypeDeclarationPattern      = regexp.MustCompile(`^\s*(?:template[ \t]*<[^>\n]+>[ \t]*)*(class|struct)[ \t]+([A-Za-z_]\w*)\b`)
	cppNamespaceDeclarationPattern = regexp.MustCompile(`^\s*(?:export[ \t]+)?namespace[ \t]+([A-Za-z_]\w*(?:::[A-Za-z_]\w*)*)\b`)
	cppAccessSpecifierPattern      = regexp.MustCompile(`^\s*(public|protected|private)\s*:\s*$`)
)

func cppTargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	return support.ScanCPPFiles(env, target, "design", func(file string, data []byte) []core.Finding {
		return cppDesignFindingsForFile(env, file, data)
	})
}

func cppDesignFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := cppGenericModuleNameFindings(env, file)
	parsed := support.ParseCLike(string(data), support.CLikeCPP)
	findings = append(findings, cppDeclFindings(env, file, parsed)...)
	surfaces := cppTypeSurfaces(parsed.Source)
	cppRecordOutOfLineMethods(surfaces, parsed.Functions)
	for _, surface := range cppSortedTypeSurfaces(surfaces) {
		if count := len(surface.methods); count > env.Config.Checks.DesignRules.MaxMethodsPerType {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID: "design.cpp.max-methods-per-type", Level: "warn", Path: file, Line: surface.line, Column: 1,
				Message: fmt.Sprintf("C++ type %s has %d methods in this file; max is %d", surface.name, count, env.Config.Checks.DesignRules.MaxMethodsPerType),
			}))
		}
		if !isCPPContractPath(file) || len(surface.publicMethods) <= env.Config.Checks.DesignRules.MaxInterfaceMethods {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID: "design.cpp.max-interface-methods", Level: "warn", Path: file, Line: surface.line, Column: 1,
			Message: fmt.Sprintf("C++ type %s exposes %d public methods in this contract; max is %d", surface.name, len(surface.publicMethods), env.Config.Checks.DesignRules.MaxInterfaceMethods),
		}))
	}
	return findings
}

func cppDeclFindings(env support.Context, file string, parsed *support.ParsedFile) []core.Finding {
	count := cppDeclarationCount(parsed)
	if count <= env.Config.Checks.DesignRules.MaxDeclsPerFile {
		return nil
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "design.cpp.max-decls-per-file",
		Level:   "warn",
		Path:    file,
		Line:    1,
		Column:  1,
		Message: fmt.Sprintf("C++ file has %d top-level declarations; max is %d", count, env.Config.Checks.DesignRules.MaxDeclsPerFile),
	})}
}

func cppDeclarationCount(parsed *support.ParsedFile) int {
	if parsed == nil {
		return 0
	}
	count := len(parsed.Functions)
	for _, line := range strings.Split(parsed.Masked, "\n") {
		if cppTypeDeclarationPattern.MatchString(line) {
			count++
		}
	}
	return count
}

func cppGenericModuleNameFindings(env support.Context, file string) []core.Finding {
	name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	for _, forbidden := range env.Config.Checks.DesignRules.ForbiddenPackageNames {
		if strings.EqualFold(name, forbidden) {
			return []core.Finding{env.NewFinding(support.FindingInput{
				RuleID: "design.cpp.generic-module-name", Level: "warn", Path: file, Line: 1, Column: 1,
				Message: fmt.Sprintf("C++ file name %q is too generic", name),
			})}
		}
	}
	return nil
}

type cppTypeSurface struct {
	name          string
	typeName      string
	line          int
	methods       map[string]struct{}
	publicMethods map[string]struct{}
}
