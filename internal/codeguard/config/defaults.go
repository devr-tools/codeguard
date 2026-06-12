package config

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func ApplyDefaults(cfg *core.Config) {
	def := defaultConfigForProfile(cfg.Profile)

	applyRootDefaults(cfg, def)
	applyCheckDefaults(cfg, def)
	applyRulePackDefaults(cfg)
}

func defaultConfigForProfile(profile string) core.Config {
	def := baseExampleConfig()
	normalized := normalizeProfile(profile)
	if spec, ok := profileCatalog[normalized]; ok {
		spec.apply(&def)
		def.Profile = normalized
	}
	return def
}

func applyRootDefaults(cfg *core.Config, def core.Config) {
	if cfg.Name == "" {
		cfg.Name = def.Name
	}
	if cfg.Profile == "" {
		cfg.Profile = def.Profile
	} else {
		cfg.Profile = normalizeProfile(cfg.Profile)
	}
	if cfg.Output.Format == "" {
		cfg.Output.Format = def.Output.Format
	}
	if cfg.Cache.Enabled == nil {
		cfg.Cache.Enabled = boolPtr(true)
	}
	if cfg.Cache.Path == "" {
		cfg.Cache.Path = def.Cache.Path
	}
}

func applyCheckDefaults(cfg *core.Config, def core.Config) {
	applyQualityDefaults(&cfg.Checks.QualityRules, def.Checks.QualityRules)
	applyDesignDefaults(&cfg.Checks.DesignRules, def.Checks.DesignRules)
	applyPromptDefaults(&cfg.Checks.PromptRules, def.Checks.PromptRules)
	applyCIDefaults(&cfg.Checks.CIRules, def.Checks.CIRules)
	applySecurityDefaults(&cfg.Checks.SecurityRules, def.Checks.SecurityRules)
}

func applyQualityDefaults(dst *core.QualityRulesConfig, def core.QualityRulesConfig) {
	if dst.MaxFileLines == 0 {
		dst.MaxFileLines = def.MaxFileLines
	}
	if dst.MaxFunctionLines == 0 {
		dst.MaxFunctionLines = def.MaxFunctionLines
	}
	if dst.MaxParameters == 0 {
		dst.MaxParameters = def.MaxParameters
	}
	if dst.MaxCyclomaticComplexity == 0 {
		dst.MaxCyclomaticComplexity = def.MaxCyclomaticComplexity
	}
	if dst.LanguageCommands == nil && len(def.LanguageCommands) > 0 {
		dst.LanguageCommands = cloneCommandCheckMap(def.LanguageCommands)
	}
	applyCoverageDeltaDefaults(&dst.CoverageDelta)
}

func applyDesignDefaults(dst *core.DesignRulesConfig, def core.DesignRulesConfig) {
	if dst.MaxDeclsPerFile == 0 {
		dst.MaxDeclsPerFile = def.MaxDeclsPerFile
	}
	if dst.MaxMethodsPerType == 0 {
		dst.MaxMethodsPerType = def.MaxMethodsPerType
	}
	if dst.MaxInterfaceMethods == 0 {
		dst.MaxInterfaceMethods = def.MaxInterfaceMethods
	}
	if dst.ForbiddenPackageNames == nil {
		dst.ForbiddenPackageNames = append([]string(nil), def.ForbiddenPackageNames...)
	}
	if dst.RequireCmdThroughInternalCLI == nil {
		dst.RequireCmdThroughInternalCLI = boolPtr(true)
	}
	if dst.ForbidInternalImportCmd == nil {
		dst.ForbidInternalImportCmd = boolPtr(true)
	}
	if dst.ForbidServiceImportInternal == nil {
		dst.ForbidServiceImportInternal = boolPtr(true)
	}
	if dst.ForbidServiceImportCmd == nil {
		dst.ForbidServiceImportCmd = boolPtr(true)
	}
	if dst.LanguageCommands == nil && len(def.LanguageCommands) > 0 {
		dst.LanguageCommands = cloneCommandCheckMap(def.LanguageCommands)
	}
}

func applyPromptDefaults(dst *core.PromptRulesConfig, def core.PromptRulesConfig) {
	if dst.FileExtensions == nil {
		dst.FileExtensions = append([]string(nil), def.FileExtensions...)
	}
	if dst.PathContains == nil {
		dst.PathContains = append([]string(nil), def.PathContains...)
	}
	if dst.ForbidSecretInterpolation == nil {
		dst.ForbidSecretInterpolation = boolPtr(true)
	}
	if dst.ForbidUnsafeInstructions == nil {
		dst.ForbidUnsafeInstructions = boolPtr(true)
	}
}

func applyCIDefaults(dst *core.CIRulesConfig, def core.CIRulesConfig) {
	if dst.RequireWorkflowDir == nil {
		dst.RequireWorkflowDir = boolPtr(true)
	}
	if dst.RequiredWorkflowFiles == nil {
		dst.RequiredWorkflowFiles = append([]string(nil), def.RequiredWorkflowFiles...)
	}
	if dst.WorkflowContentRules == nil {
		dst.WorkflowContentRules = append([]core.WorkflowRuleConfig(nil), def.WorkflowContentRules...)
	}
	if dst.RequiredReleaseFiles == nil && len(def.RequiredReleaseFiles) > 0 {
		dst.RequiredReleaseFiles = append([]string(nil), def.RequiredReleaseFiles...)
	}
	if dst.RequiredAutomationPaths == nil && len(def.RequiredAutomationPaths) > 0 {
		dst.RequiredAutomationPaths = append([]string(nil), def.RequiredAutomationPaths...)
	}
	if dst.AllowedTestPaths == nil && len(def.AllowedTestPaths) > 0 {
		dst.AllowedTestPaths = append([]string(nil), def.AllowedTestPaths...)
	}
	applyTestQualityDefaults(&dst.TestQuality)
}

func applySecurityDefaults(dst *core.SecurityRulesConfig, def core.SecurityRulesConfig) {
	if dst.GovulncheckMode == "" {
		dst.GovulncheckMode = def.GovulncheckMode
	}
	if dst.GovulncheckCommand == "" {
		dst.GovulncheckCommand = def.GovulncheckCommand
	}
	if dst.LanguageCommands == nil && len(def.LanguageCommands) > 0 {
		dst.LanguageCommands = cloneCommandCheckMap(def.LanguageCommands)
	}
}

func applyRulePackDefaults(cfg *core.Config) {
	for packIdx := range cfg.RulePacks {
		for ruleIdx := range cfg.RulePacks[packIdx].Rules {
			rule := &cfg.RulePacks[packIdx].Rules[ruleIdx]
			if strings.TrimSpace(rule.Section) == "" {
				rule.Section = "Custom Rules"
			}
			if strings.TrimSpace(rule.Severity) == "" {
				rule.Severity = "warn"
			}
		}
	}
}

func cloneCommandCheckMap(src map[string][]core.CommandCheckConfig) map[string][]core.CommandCheckConfig {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string][]core.CommandCheckConfig, len(src))
	for language, checks := range src {
		dst[language] = append([]core.CommandCheckConfig(nil), checks...)
	}
	return dst
}
