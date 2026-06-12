package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

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
	if dst.DetectNPlusOneQuery == nil {
		dst.DetectNPlusOneQuery = boolPtr(true)
	}
	if dst.DetectAllocInLoop == nil {
		dst.DetectAllocInLoop = boolPtr(true)
	}
	if dst.DetectSyncIOInHandlers == nil {
		dst.DetectSyncIOInHandlers = boolPtr(true)
	}
	if dst.DetectUnboundedConcurrency == nil {
		dst.DetectUnboundedConcurrency = boolPtr(true)
	}
	if dst.CloneTokenThreshold == 0 {
		dst.CloneTokenThreshold = def.CloneTokenThreshold
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
	if dst.GodModuleThreshold == 0 {
		dst.GodModuleThreshold = def.GodModuleThreshold
	}
	if dst.HighImpactChangeThreshold == 0 {
		dst.HighImpactChangeThreshold = def.HighImpactChangeThreshold
	}
	if dst.DetectImportCycles == nil {
		dst.DetectImportCycles = boolPtr(true)
	}
	if dst.DetectGodModules == nil {
		dst.DetectGodModules = boolPtr(true)
	}
	if dst.DetectHighImpactChanges == nil {
		dst.DetectHighImpactChanges = boolPtr(true)
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
