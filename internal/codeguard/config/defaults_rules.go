package config

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func applyQualityDefaults(dst *core.QualityRulesConfig, def core.QualityRulesConfig) {
	defaultInt(&dst.MaxFileLines, def.MaxFileLines)
	defaultInt(&dst.MaxFunctionLines, def.MaxFunctionLines)
	defaultInt(&dst.MaxParameters, def.MaxParameters)
	defaultInt(&dst.MaxCyclomaticComplexity, def.MaxCyclomaticComplexity)
	defaultInt(&dst.CloneTokenThreshold, def.CloneTokenThreshold)
	defaultCommandMap(&dst.LanguageCommands, def.LanguageCommands)
	applyAIChangeRiskDefaults(&dst.AIChangeRisk, def.AIChangeRisk)
	applyRiskScoringDefaults(&dst.RiskScoring)
	applyCoverageDeltaDefaults(&dst.CoverageDelta)
	applyCPPToolingDefaults(&dst.CPPTooling)
	defaultBoolPtr(&dst.LocalPrecision, boolValueOrTrue(def.LocalPrecision))
}

func applyRiskScoringDefaults(dst *core.RiskScoringConfig) {
	defaultBoolPtr(&dst.Enabled, true)
	defaultInt(&dst.MaxHotspots, 5)
	defaultInt(&dst.ChangedFileWeight, 5)
	defaultInt(&dst.FailFindingWeight, 30)
	defaultInt(&dst.WarnFindingWeight, 15)
	defaultInt(&dst.SecurityWeight, 10)
	defaultInt(&dst.SupplyChainWeight, 10)
	defaultInt(&dst.CoverageGapWeight, 15)
	defaultInt(&dst.AIProvenanceWeight, 15)
	defaultInt(&dst.AISignalWeight, 10)
	defaultInt(&dst.SlopScoreDivisor, 10)
}

func applyCPPToolingDefaults(dst *core.CPPToolingConfig) {
	dst.ClangFormatMode = strings.ToLower(strings.TrimSpace(dst.ClangFormatMode))
	if dst.ClangFormatMode == "" {
		dst.ClangFormatMode = core.ExternalToolModeOff
	}
	dst.CompilerMode = strings.ToLower(strings.TrimSpace(dst.CompilerMode))
	if dst.CompilerMode == "" {
		dst.CompilerMode = core.ExternalToolModeOff
	}
	if strings.TrimSpace(dst.ClangFormatCommand) == "" {
		dst.ClangFormatCommand = "clang-format"
	}
	if strings.TrimSpace(dst.CompilerCommand) == "" {
		dst.CompilerCommand = "clang++"
	}
}

func applyPerformanceDefaults(dst *core.PerformanceRulesConfig) {
	applyDefaultBoolPtrs(
		&dst.DetectNPlusOneQuery,
		&dst.DetectAllocInLoop,
		&dst.DetectSyncIOInHandlers,
		&dst.DetectUnboundedConcurrency,
		&dst.DetectRegexCompileInLoop,
		&dst.DetectDeferInLoop,
		&dst.DetectSleepInLoop,
		&dst.DetectAwaitInLoop,
		&dst.DetectTimerLeaks,
		&dst.DetectUnboundedReads,
		&dst.DetectComplexityRegression,
		&dst.DetectHotPathPatterns,
		&dst.DetectFrameworkPatterns,
		&dst.DetectRebuildCascade,
	)
	defaultBoolPtr(&dst.DetectPreallocInLoop, false)
	applyPerformanceMeasurementDefaults(dst)
	applyPerformanceGraphDefaults(dst)
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
	if dst.RequiredGates == nil {
		dst.RequiredGates = append([]string(nil), def.RequiredGates...)
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
	applyTaintDefaults(dst)
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

func applyTaintDefaults(dst *core.SecurityRulesConfig) {
	defaultBoolPtr(&dst.TaintGo, true)
	defaultBoolPtr(&dst.TaintPython, true)
	defaultBoolPtr(&dst.TaintCPP, true)
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
		&dst.DetectUndocumentedCommands,
		&dst.DetectOversizedAgentDocs,
		&dst.DetectDocLinkRot,
	)
	defaultInt(&dst.MaxFileLines, def.MaxFileLines)
	defaultInt(&dst.AmbiguousSymbolThreshold, def.AmbiguousSymbolThreshold)
	defaultInt(&dst.MaxAgentDocLines, def.MaxAgentDocLines)
}

func applySupplyChainDefaults(dst *core.SupplyChainRulesConfig, def core.SupplyChainRulesConfig) {
	defaultBoolPtr(&dst.RequireLockfile, boolValueOrTrue(def.RequireLockfile))
	defaultBoolPtr(&dst.DetectLockfileDrift, boolValueOrTrue(def.DetectLockfileDrift))
	defaultBoolPtr(&dst.DetectUnpinned, boolValueOrTrue(def.DetectUnpinned))
	defaultBoolPtr(&dst.DetectProvenance, boolValueOrTrue(def.DetectProvenance))
	// Vulnerability matching is opt-in: a repository must choose and maintain
	// the advisory cache it trusts.
	defaultBoolPtr(&dst.DetectVulnerabilities, false)
	defaultStringSlice(&dst.AllowedLicenses, def.AllowedLicenses, false)
	defaultStringSlice(&dst.DeniedLicenses, def.DeniedLicenses, false)
	defaultSingleCommandMap(&dst.LicenseCommands, def.LicenseCommands)
}

func applyDeliveryDefaults(dst *core.DeliveryRulesConfig, def core.DeliveryRulesConfig) {
	applyDefaultBoolPtrs(
		&dst.DetectMissingRollbackStrategy,
		&dst.DetectUnsafeMigrationOrder,
		&dst.DetectHighRiskChangeWithoutKillSwitch,
		&dst.DetectMissingPostDeployVerification,
	)
	defaultStringSlice(&dst.RollbackEvidencePatterns, def.RollbackEvidencePatterns, false)
	defaultStringSlice(&dst.KillSwitchPatterns, def.KillSwitchPatterns, false)
	defaultStringSlice(&dst.PostDeployVerificationPatterns, def.PostDeployVerificationPatterns, false)
	defaultStringSlice(&dst.MigrationPathPatterns, def.MigrationPathPatterns, false)
	defaultStringSlice(&dst.HighRiskPathPatterns, def.HighRiskPathPatterns, false)
	defaultStringSlice(&dst.BootstrapPathPatterns, def.BootstrapPathPatterns, false)
}

func applyReliabilityDefaults(dst *core.ReliabilityRulesConfig, def core.ReliabilityRulesConfig) {
	applyDefaultBoolPtrs(
		&dst.DetectMissingTimeout,
		&dst.DetectUnboundedRetry,
		&dst.DetectRetryWithoutBackoff,
		&dst.DetectNonIdempotentRetry,
		&dst.DetectMissingCancellation,
		&dst.DetectUnboundedWork,
		&dst.DetectMissingConcurrencyLimit,
		&dst.DetectResourceLeak,
		&dst.DetectPartialFailureHidden,
		&dst.DetectMissingGracefulShutdown,
		&dst.DetectSwallowedError,
		&dst.DetectLostErrorContext,
		&dst.DetectRecoverablePanic,
	)
	defaultInt(&dst.MaxRetryAttempts, def.MaxRetryAttempts)
	defaultInt(&dst.MaxInlineGoroutinesPerFunction, def.MaxInlineGoroutinesPerFunction)
}

func applyDataDefaults(dst *core.DataRulesConfig, def core.DataRulesConfig) {
	applyDefaultBoolPtrs(
		&dst.DetectReadModifyWriteRace,
		&dst.DetectMissingTransaction,
		&dst.DetectSideEffectInTransaction,
		&dst.DetectNonIdempotentConsumer,
		&dst.DetectMissingDeduplication,
		&dst.DetectUnsafeDualWrite,
		&dst.DetectMissingOutboxStrategy,
		&dst.DetectUnstablePagination,
		&dst.DetectUnboundedRead,
		&dst.DetectExactlyOnceAssumption,
		&dst.DetectCacheWithoutPolicy,
	)
	defaultInt(&dst.MaxUnboundedReadRows, def.MaxUnboundedReadRows)
	defaultInt(&dst.MaxWritesWithoutTransaction, def.MaxWritesWithoutTransaction)
}

func applyObservabilityDefaults(dst *core.ObservabilityRulesConfig, def core.ObservabilityRulesConfig) {
	applyDefaultBoolPtrs(
		&dst.DetectUnstructuredLog,
		&dst.DetectErrorWithoutContext,
		&dst.DetectSensitiveLogData,
		&dst.DetectHighCardinalityLabel,
		&dst.DetectCriticalPathUninstrumented,
		&dst.DetectLogAndIgnore,
		&dst.DetectShallowHealthCheck,
	)
	defaultStringSlice(&dst.StructuredLoggerPatterns, def.StructuredLoggerPatterns, false)
	defaultStringSlice(&dst.SensitiveNamePatterns, def.SensitiveNamePatterns, false)
	defaultStringSlice(&dst.HighCardinalityLabelPatterns, def.HighCardinalityLabelPatterns, false)
	defaultStringSlice(&dst.CriticalPathPatterns, def.CriticalPathPatterns, false)
	defaultStringSlice(&dst.HealthcheckPathPatterns, def.HealthcheckPathPatterns, false)
	defaultStringSlice(&dst.InstrumentationEvidencePatterns, def.InstrumentationEvidencePatterns, false)
}

func applyOperationsDefaults(dst *core.OperationsRulesConfig, def core.OperationsRulesConfig) {
	applyDefaultBoolPtrs(&dst.DetectMissingOwner, &dst.DetectMissingRunbook)
	defaultStringSlice(&dst.OwnerFilePatterns, def.OwnerFilePatterns, false)
	defaultStringSlice(&dst.RunbookPathPatterns, def.RunbookPathPatterns, false)
	defaultStringSlice(&dst.CriticalPathPatterns, def.CriticalPathPatterns, false)
}

func applyChangeDefaults(dst *core.ChangeRulesConfig, def core.ChangeRulesConfig) {
	applyDefaultBoolPtrs(
		&dst.DetectBehaviorChangeWithoutTest,
		&dst.DetectFailurePathMissing,
		&dst.DetectHardwiredDependency,
		&dst.DetectNondeterministicDomain,
		&dst.DetectLegacyHotspotUncovered,
		&dst.DetectMixedConcerns,
		&dst.DetectOversizedDiff,
		&dst.DetectMixedRefactorAndBehavior,
		&dst.DetectTooManyConcerns,
		&dst.DetectUnnecessarySurfaceArea,
		&dst.DetectOneUseAbstraction,
		&dst.DetectDuplicateHelper,
		&dst.DetectCleanupRegression,
		&dst.DetectComplexityIncreased,
		&dst.DetectMoveWithoutVerification,
		&dst.DetectRefactorBehaviorChange,
		&dst.DetectRefactorPublicContract,
		&dst.DetectRefactorTestCoverageDrop,
		&dst.DetectRefactorErrorPathChange,
		&dst.DetectRefactorSideEffectReorder,
		&dst.DetectRefactorVisibilityExpand,
		&dst.DetectRefactorDependencyWorsened,
		&dst.DetectRefactorDuplicateLeftBehind,
		&dst.DetectRefactorDeadPathLeftBehind,
	)
	defaultInt(&dst.MaxChangedFiles, def.MaxChangedFiles)
	defaultInt(&dst.MaxChangedDirectories, def.MaxChangedDirectories)
	defaultInt(&dst.MaxChangedLines, def.MaxChangedLines)
	defaultInt(&dst.MaxPublicInterfacesChanged, def.MaxPublicInterfacesChanged)
	defaultInt(&dst.MaxConcernFamilies, def.MaxConcernFamilies)
	defaultInt(&dst.MinTestToProductionRatioPercent, def.MinTestToProductionRatioPercent)
}

func applyProductionRiskDefaults(dst *core.ProductionRiskConfig, def core.ProductionRiskConfig) {
	defaultBoolPtr(&dst.Enabled, boolValueOrTrue(def.Enabled))
	defaultInt(&dst.WarnThreshold, def.WarnThreshold)
	defaultInt(&dst.FailThreshold, def.FailThreshold)
	defaultInt(&dst.ReliabilityWeight, def.ReliabilityWeight)
	defaultInt(&dst.DataWeight, def.DataWeight)
	defaultInt(&dst.FailWeight, def.FailWeight)
	defaultInt(&dst.WarnWeight, def.WarnWeight)
}
