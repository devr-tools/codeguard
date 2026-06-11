package design

import (
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

type designRules struct {
	requireCmdThroughInternalCLI bool
	forbidInternalImportCmd      bool
	forbidServiceImportInternal  bool
	forbidServiceImportCmd       bool
	maxDeclsPerFile              int
	maxMethodsPerType            int
	maxInterfaceMethods          int
	forbiddenPackageNames        []string
}

func resolveDesignRules(cfg core.DesignRulesConfig) designRules {
	rules := designRules{
		requireCmdThroughInternalCLI: true,
		forbidInternalImportCmd:      true,
		forbidServiceImportInternal:  true,
		forbidServiceImportCmd:       true,
		maxDeclsPerFile:              12,
		maxMethodsPerType:            8,
		maxInterfaceMethods:          5,
		forbiddenPackageNames:        []string{"util", "utils", "common", "helpers", "misc"},
	}
	rules.requireCmdThroughInternalCLI = boolValue(cfg.RequireCmdThroughInternalCLI, rules.requireCmdThroughInternalCLI)
	rules.forbidInternalImportCmd = boolValue(cfg.ForbidInternalImportCmd, rules.forbidInternalImportCmd)
	rules.forbidServiceImportInternal = boolValue(cfg.ForbidServiceImportInternal, rules.forbidServiceImportInternal)
	rules.forbidServiceImportCmd = boolValue(cfg.ForbidServiceImportCmd, rules.forbidServiceImportCmd)
	if cfg.MaxDeclsPerFile > 0 {
		rules.maxDeclsPerFile = cfg.MaxDeclsPerFile
	}
	if cfg.MaxMethodsPerType > 0 {
		rules.maxMethodsPerType = cfg.MaxMethodsPerType
	}
	if cfg.MaxInterfaceMethods > 0 {
		rules.maxInterfaceMethods = cfg.MaxInterfaceMethods
	}
	if len(cfg.ForbiddenPackageNames) > 0 {
		rules.forbiddenPackageNames = normalizeNames(cfg.ForbiddenPackageNames)
	}
	return rules
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func normalizeNames(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
