package design

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func cppTargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	return support.ScanCPPFiles(env, target, "design", func(file string, data []byte) []core.Finding {
		return cppDesignFindingsForFile(env, file, data)
	})
}

func cppDesignFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := cppGenericModuleNameFindings(env, file)
	parsed := support.ParseCLike(string(data), support.CLikeCPP)
	counts := make(map[string]int)
	for _, function := range parsed.Functions {
		separator := strings.LastIndex(function.Name, "::")
		if separator <= 0 {
			continue
		}
		counts[function.Name[:separator]]++
	}
	for typeName, count := range counts {
		if count <= env.Config.Checks.DesignRules.MaxMethodsPerType {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID: "design.cpp.max-methods-per-type", Level: "warn", Path: file, Line: 1, Column: 1,
			Message: fmt.Sprintf("C++ type %s has %d out-of-line methods in this file; max is %d", typeName, count, env.Config.Checks.DesignRules.MaxMethodsPerType),
		}))
	}
	return findings
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
