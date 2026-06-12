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
	if cfg.AI.Enabled == nil {
		cfg.AI.Enabled = boolPtr(false)
	}
	if cfg.AI.Cache.Path == "" {
		cfg.AI.Cache.Path = def.AI.Cache.Path
	}
}

func applyCheckDefaults(cfg *core.Config, def core.Config) {
	applyQualityDefaults(&cfg.Checks.QualityRules, def.Checks.QualityRules)
	applyDesignDefaults(&cfg.Checks.DesignRules, def.Checks.DesignRules)
	applyPromptDefaults(&cfg.Checks.PromptRules, def.Checks.PromptRules)
	applyCIDefaults(&cfg.Checks.CIRules, def.Checks.CIRules)
	applySecurityDefaults(&cfg.Checks.SecurityRules, def.Checks.SecurityRules)
	applyAIDefaults(&cfg.AI, def.AI)
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
	if dst.CloneTokenThreshold == 0 {
		dst.CloneTokenThreshold = def.CloneTokenThreshold
	}
	if dst.LanguageCommands == nil && len(def.LanguageCommands) > 0 {
		dst.LanguageCommands = cloneCommandCheckMap(def.LanguageCommands)
	}
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
	applyDefaultBoolPtrs(
		&dst.RequireCmdThroughInternalCLI,
		&dst.ForbidInternalImportCmd,
		&dst.ForbidServiceImportInternal,
		&dst.ForbidServiceImportCmd,
	)
	if dst.LanguageCommands == nil && len(def.LanguageCommands) > 0 {
		dst.LanguageCommands = cloneCommandCheckMap(def.LanguageCommands)
	}
	if dst.LanguageDiffCommands == nil && len(def.LanguageDiffCommands) > 0 {
		dst.LanguageDiffCommands = cloneCommandCheckMap(def.LanguageDiffCommands)
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

func applyAIDefaults(dst *core.AIConfig, def core.AIConfig) {
	if dst.Provider.Type == "" {
		dst.Provider.Type = def.Provider.Type
	}
	if dst.Provider.Model == "" {
		dst.Provider.Model = def.Provider.Model
	}
	if dst.Provider.BaseURL == "" {
		dst.Provider.BaseURL = def.Provider.BaseURL
	}
	if dst.Provider.APIKeyEnv == "" {
		dst.Provider.APIKeyEnv = def.Provider.APIKeyEnv
	}
	if dst.HybridTriage.Enabled == nil {
		dst.HybridTriage.Enabled = boolPtr(true)
	}
	if dst.HybridTriage.SuppressDismissed == nil {
		dst.HybridTriage.SuppressDismissed = boolPtr(true)
	}
	if dst.HybridTriage.CandidateSections == nil {
		dst.HybridTriage.CandidateSections = append([]string(nil), def.HybridTriage.CandidateSections...)
	}
	if dst.HybridTriage.CandidateSeverities == nil {
		dst.HybridTriage.CandidateSeverities = append([]string(nil), def.HybridTriage.CandidateSeverities...)
	}
	if dst.Semantic.Enabled == nil {
		dst.Semantic.Enabled = boolPtr(true)
	}
	if dst.Semantic.FunctionContract == nil {
		dst.Semantic.FunctionContract = boolPtr(true)
	}
	if dst.Semantic.MisleadingErrorMessages == nil {
		dst.Semantic.MisleadingErrorMessages = boolPtr(true)
	}
	if dst.Semantic.TestBehaviorCoverage == nil {
		dst.Semantic.TestBehaviorCoverage = boolPtr(true)
	}
	if dst.AutoFix.Enabled == nil {
		dst.AutoFix.Enabled = boolPtr(false)
	}
	if dst.AutoFix.VerifyTests == nil {
		dst.AutoFix.VerifyTests = boolPtr(true)
	}
	if dst.AutoFix.MaxFixes == 0 {
		dst.AutoFix.MaxFixes = def.AutoFix.MaxFixes
	}
	if dst.AutoFix.TestCommands == nil && len(def.AutoFix.TestCommands) > 0 {
		dst.AutoFix.TestCommands = append([]core.CommandCheckConfig(nil), def.AutoFix.TestCommands...)
	}
}
