package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

func applyQualityDefaults(dst *core.QualityRulesConfig, def core.QualityRulesConfig) {
	defaultInt(&dst.MaxFileLines, def.MaxFileLines)
	defaultInt(&dst.MaxFunctionLines, def.MaxFunctionLines)
	defaultInt(&dst.MaxParameters, def.MaxParameters)
	defaultInt(&dst.MaxCyclomaticComplexity, def.MaxCyclomaticComplexity)
	defaultInt(&dst.CloneTokenThreshold, def.CloneTokenThreshold)
	applyDefaultBoolPtrs(
		&dst.DetectNPlusOneQuery,
		&dst.DetectAllocInLoop,
		&dst.DetectSyncIOInHandlers,
		&dst.DetectUnboundedConcurrency,
	)
	defaultBoolPtr(&dst.DetectPreallocInLoop, false)
	defaultCommandMap(&dst.LanguageCommands, def.LanguageCommands)
	applyAIChangeRiskDefaults(&dst.AIChangeRisk, def.AIChangeRisk)
	applyCoverageDeltaDefaults(&dst.CoverageDelta)
}

func applyDesignDefaults(dst *core.DesignRulesConfig, def core.DesignRulesConfig) {
	defaultInt(&dst.MaxDeclsPerFile, def.MaxDeclsPerFile)
	defaultInt(&dst.MaxMethodsPerType, def.MaxMethodsPerType)
	defaultInt(&dst.MaxInterfaceMethods, def.MaxInterfaceMethods)
	defaultInt(&dst.GodModuleThreshold, def.GodModuleThreshold)
	defaultInt(&dst.HighImpactChangeThreshold, def.HighImpactChangeThreshold)
	defaultStringSlice(&dst.ForbiddenPackageNames, def.ForbiddenPackageNames, false)
	applyDefaultBoolPtrs(
		&dst.DetectImportCycles,
		&dst.DetectGodModules,
		&dst.DetectHighImpactChanges,
		&dst.RequireCmdThroughInternalCLI,
		&dst.ForbidInternalImportCmd,
		&dst.ForbidServiceImportInternal,
		&dst.ForbidServiceImportCmd,
	)
	defaultCommandMap(&dst.LanguageCommands, def.LanguageCommands)
	defaultCommandMap(&dst.LanguageDiffCommands, def.LanguageDiffCommands)
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
	if dst.TaintGo == nil {
		dst.TaintGo = boolPtr(true)
	}
	if dst.TaintPython == nil {
		dst.TaintPython = boolPtr(true)
	}
	if dst.DemoteFixtureFindings == nil {
		dst.DemoteFixtureFindings = boolPtr(true)
	}
	if dst.TypeScriptTaintMaxDepth == 0 {
		dst.TypeScriptTaintMaxDepth = def.TypeScriptTaintMaxDepth
	}
	if dst.GovulncheckCommand == "" {
		dst.GovulncheckCommand = def.GovulncheckCommand
	}
	if dst.LanguageCommands == nil && len(def.LanguageCommands) > 0 {
		dst.LanguageCommands = cloneCommandCheckMap(def.LanguageCommands)
	}
	if dst.Secrets == nil {
		dst.Secrets = &core.SecretsRulesConfig{}
	}
	if dst.Secrets.Enabled == nil {
		dst.Secrets.Enabled = boolPtr(true)
	}
}

func applyAIChangeRiskDefaults(dst *core.AIChangeRiskConfig, def core.AIChangeRiskConfig) {
	defaultBoolPtr(&dst.Enabled, boolValueOrTrue(def.Enabled))
	if dst.WarnThreshold == 0 {
		dst.WarnThreshold = def.WarnThreshold
	}
	if dst.FailThreshold == 0 {
		dst.FailThreshold = def.FailThreshold
	}
}

func applyContextDefaults(dst *core.ContextRulesConfig, def core.ContextRulesConfig) {
	applyDefaultBoolPtrs(
		&dst.DetectMissingAgentDocs,
		&dst.DetectAgentDocsDrift,
		&dst.DetectReadmeDrift,
		&dst.DetectOversizedFiles,
		&dst.DetectAmbiguousSymbols,
	)
	defaultInt(&dst.MaxFileLines, def.MaxFileLines)
	defaultInt(&dst.AmbiguousSymbolThreshold, def.AmbiguousSymbolThreshold)
}

func applySupplyChainDefaults(dst *core.SupplyChainRulesConfig, def core.SupplyChainRulesConfig) {
	defaultBoolPtr(&dst.RequireLockfile, boolValueOrTrue(def.RequireLockfile))
	defaultBoolPtr(&dst.DetectLockfileDrift, boolValueOrTrue(def.DetectLockfileDrift))
	defaultBoolPtr(&dst.DetectUnpinned, boolValueOrTrue(def.DetectUnpinned))
	defaultStringSlice(&dst.AllowedLicenses, def.AllowedLicenses, false)
	defaultStringSlice(&dst.DeniedLicenses, def.DeniedLicenses, false)
	defaultSingleCommandMap(&dst.LicenseCommands, def.LicenseCommands)
}
