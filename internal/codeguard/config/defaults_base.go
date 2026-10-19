package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// baseDefaultConfig is the conservative policy baseline that ApplyDefaults
// fills omitted configuration keys from. Every value here silently becomes the
// effective policy for any repository that does not set that key, so it must
// hold only settings that are defensible for ANY repository: scan mechanics
// (cache/output paths, parser mode), section activation that is off or
// scan-mode derived, and thresholds that have no meaningful "unset" value
// (a max of 0 would fail everything).
//
// It deliberately holds NO repository-shape policy — no required filenames, no
// workflow content markers, no test-directory layout, no package-layering
// rules. codeguard cannot know those apply. Such policy belongs in
// applyStarterPolicy (see example.go), which only `codeguard init` writes, or
// in a named profile, which a repository opts into explicitly.
//
// Adding a value here is a decision to enforce it on every codeguard user.
func baseDefaultConfig() core.Config {
	return core.Config{
		Name:    "codeguard-default",
		Checks:  baseDefaultChecks(),
		AI:      baseDefaultAIConfig(),
		Output:  core.OutputConfig{Format: "text"},
		Cache:   baseDefaultCacheConfig(),
		Parsers: core.ParsersConfig{TreeSitter: core.TreeSitterModeOff},
	}
}

// baseDefaultChecks holds section activation. The plain-bool sections
// (quality, design, security, prompts, ci, supply_chain) are NOT defaulted by
// applyCheckDefaults — an omitted key means off — so their value here is only
// a record of the recommended set; applyCheckActivationDefaults
// (checks.use_recommended_defaults) and the starter config are what turn them
// on. The *bool sections below ARE defaulted from here.
func baseDefaultChecks() core.CheckConfig {
	return core.CheckConfig{
		Quality: false,
		Design:  false,
		// Performance is opt-in while the rules settle into their own section;
		// they previously ran (enabled) inside quality under quality.* ids. The
		// explicit false (vs nil) writes the key into generated configs so new
		// users discover it, and suppresses the upgrade hint in scan output.
		Performance:        boolPtr(false),
		Security:           false,
		Prompts:            false,
		CI:                 false,
		SupplyChain:        false,
		Delivery:           boolPtr(false),
		Reliability:        boolPtr(false),
		Data:               boolPtr(false),
		Observability:      boolPtr(false),
		Operations:         boolPtr(false),
		Change:             boolPtr(false),
		QualityRules:       baseDefaultQualityRules(),
		PerformanceRules:   baseDefaultPerformanceRules(),
		DesignRules:        baseDefaultDesignRules(),
		PromptRules:        baseDefaultPromptRules(),
		CIRules:            baseDefaultCIRules(),
		DeliveryRules:      baseDefaultDeliveryRules(),
		SecurityRules:      baseDefaultSecurityRules(),
		SupplyChainRules:   baseDefaultSupplyChainRules(),
		ReliabilityRules:   baseDefaultReliabilityRules(),
		DataRules:          baseDefaultDataRules(),
		ObservabilityRules: baseDefaultObservabilityRules(),
		OperationsRules:    baseDefaultOperationsRules(),
		ChangeRules:        baseDefaultChangeRules(),
		ContractRules:      baseDefaultContractRules(),
		ContextRules:       baseDefaultContextRules(),
		ProductionRisk:     baseDefaultProductionRisk(),
	}
}

func baseDefaultContextRules() core.ContextRulesConfig {
	return core.ContextRulesConfig{
		DetectMissingAgentDocs:     boolPtr(true),
		DetectAgentDocsDrift:       boolPtr(true),
		DetectReadmeDrift:          boolPtr(true),
		DetectOversizedFiles:       boolPtr(true),
		DetectAmbiguousSymbols:     boolPtr(true),
		DetectUndocumentedCommands: boolPtr(true),
		DetectOversizedAgentDocs:   boolPtr(true),
		DetectDocLinkRot:           boolPtr(true),
		MaxFileLines:               1500,
		AmbiguousSymbolThreshold:   4,
		MaxAgentDocLines:           600,
	}
}

func baseDefaultSupplyChainRules() core.SupplyChainRulesConfig {
	return core.SupplyChainRulesConfig{
		RequireLockfile:     boolPtr(true),
		DetectLockfileDrift: boolPtr(true),
		DetectUnpinned:      boolPtr(true),
		DetectProvenance:    boolPtr(true),
	}
}

func baseDefaultDeliveryRules() core.DeliveryRulesConfig {
	return core.DeliveryRulesConfig{
		DetectMissingRollbackStrategy:         boolPtr(true),
		DetectUnsafeMigrationOrder:            boolPtr(true),
		DetectHighRiskChangeWithoutKillSwitch: boolPtr(true),
		DetectMissingPostDeployVerification:   boolPtr(true),
		RollbackEvidencePatterns:              []string{"rollback", "roll back", "revert", "restore", "previous version"},
		KillSwitchPatterns:                    []string{"feature flag", "feature_flag", "kill switch", "killswitch", "rollout", "launchdarkly"},
		PostDeployVerificationPatterns:        []string{"smoke", "health", "synthetic", "post-deploy", "post deploy", "curl", "slo"},
		MigrationPathPatterns:                 []string{"migrations/**", "db/migrate/**", "alembic/**"},
		HighRiskPathPatterns:                  []string{"**/payment/**", "**/payments/**", "**/auth/**", "**/checkout/**", "**/billing/**", "**/migration/**", "**/migrations/**"},
		BootstrapPathPatterns:                 []string{"cmd/**", "config/**", "configs/**", "**/config/**", "**/bootstrap/**", "scripts/**", ".github/**"},
	}
}

func baseDefaultObservabilityRules() core.ObservabilityRulesConfig {
	return core.ObservabilityRulesConfig{
		DetectUnstructuredLog:            boolPtr(true),
		DetectErrorWithoutContext:        boolPtr(true),
		DetectSensitiveLogData:           boolPtr(true),
		DetectHighCardinalityLabel:       boolPtr(true),
		DetectCriticalPathUninstrumented: boolPtr(true),
		DetectLogAndIgnore:               boolPtr(true),
		DetectShallowHealthCheck:         boolPtr(true),
		StructuredLoggerPatterns:         []string{"logger.", "logrus.", "zap.", "slog.", "zerolog.", "structlog.", "logging."},
		SensitiveNamePatterns:            []string{"password", "passwd", "secret", "token", "api_key", "apikey", "authorization", "cookie", "ssn", "email"},
		HighCardinalityLabelPatterns:     []string{"user_id", "userid", "email", "request_id", "requestid", "trace_id", "session_id", "uuid", "path", "url"},
		CriticalPathPatterns:             []string{"handler", "controller", "consumer", "job", "worker", "payment", "checkout", "auth", "migration"},
		HealthcheckPathPatterns:          []string{"health", "healthz", "ready", "readyz", "live", "livez"},
		InstrumentationEvidencePatterns:  []string{"span", "trace", "metric", "counter", "histogram", "observe", "instrument", "prometheus"},
	}
}

func baseDefaultOperationsRules() core.OperationsRulesConfig {
	return core.OperationsRulesConfig{
		DetectMissingOwner:   boolPtr(true),
		DetectMissingRunbook: boolPtr(true),
		OwnerFilePatterns:    []string{"CODEOWNERS", ".github/CODEOWNERS", "OWNERS", "owners.yaml", "catalog-info.yaml", "service.yaml", "service.yml"},
		RunbookPathPatterns:  []string{"runbook", "runbooks", "docs/runbooks", "ops", "operations"},
		CriticalPathPatterns: []string{"cmd/", "internal/", "service", "api", "worker", "consumer", "job", "payment", "auth", "deploy", "migrations"},
	}
}

func baseDefaultReliabilityRules() core.ReliabilityRulesConfig {
	return core.ReliabilityRulesConfig{
		DetectMissingTimeout:           boolPtr(true),
		DetectUnboundedRetry:           boolPtr(true),
		DetectRetryWithoutBackoff:      boolPtr(true),
		DetectNonIdempotentRetry:       boolPtr(true),
		DetectMissingCancellation:      boolPtr(true),
		DetectUnboundedWork:            boolPtr(true),
		DetectMissingConcurrencyLimit:  boolPtr(true),
		DetectResourceLeak:             boolPtr(true),
		DetectPartialFailureHidden:     boolPtr(true),
		DetectMissingGracefulShutdown:  boolPtr(true),
		DetectSwallowedError:           boolPtr(true),
		DetectLostErrorContext:         boolPtr(true),
		DetectRecoverablePanic:         boolPtr(true),
		MaxRetryAttempts:               3,
		MaxInlineGoroutinesPerFunction: 4,
	}
}

func baseDefaultDataRules() core.DataRulesConfig {
	return core.DataRulesConfig{
		DetectReadModifyWriteRace:     boolPtr(true),
		DetectMissingTransaction:      boolPtr(true),
		DetectSideEffectInTransaction: boolPtr(true),
		DetectNonIdempotentConsumer:   boolPtr(true),
		DetectMissingDeduplication:    boolPtr(true),
		DetectUnsafeDualWrite:         boolPtr(true),
		DetectMissingOutboxStrategy:   boolPtr(true),
		DetectUnstablePagination:      boolPtr(true),
		DetectUnboundedRead:           boolPtr(true),
		DetectExactlyOnceAssumption:   boolPtr(true),
		DetectCacheWithoutPolicy:      boolPtr(true),
		MaxUnboundedReadRows:          1000,
		MaxWritesWithoutTransaction:   1,
	}
}

func baseDefaultChangeRules() core.ChangeRulesConfig {
	return core.ChangeRulesConfig{
		DetectBehaviorChangeWithoutTest:   boolPtr(true),
		DetectFailurePathMissing:          boolPtr(true),
		DetectHardwiredDependency:         boolPtr(true),
		DetectNondeterministicDomain:      boolPtr(true),
		DetectLegacyHotspotUncovered:      boolPtr(true),
		DetectMixedConcerns:               boolPtr(true),
		DetectOversizedDiff:               boolPtr(true),
		DetectMixedRefactorAndBehavior:    boolPtr(true),
		DetectTooManyConcerns:             boolPtr(true),
		DetectUnnecessarySurfaceArea:      boolPtr(true),
		DetectOneUseAbstraction:           boolPtr(true),
		DetectDuplicateHelper:             boolPtr(true),
		DetectCleanupRegression:           boolPtr(true),
		DetectComplexityIncreased:         boolPtr(true),
		DetectMoveWithoutVerification:     boolPtr(true),
		DetectRefactorBehaviorChange:      boolPtr(true),
		DetectRefactorPublicContract:      boolPtr(true),
		DetectRefactorTestCoverageDrop:    boolPtr(true),
		DetectRefactorErrorPathChange:     boolPtr(true),
		DetectRefactorSideEffectReorder:   boolPtr(true),
		DetectRefactorVisibilityExpand:    boolPtr(true),
		DetectRefactorDependencyWorsened:  boolPtr(true),
		DetectRefactorDuplicateLeftBehind: boolPtr(true),
		DetectRefactorDeadPathLeftBehind:  boolPtr(true),
		MaxChangedFiles:                   25,
		MaxChangedDirectories:             8,
		MaxChangedLines:                   800,
		MaxPublicInterfacesChanged:        3,
		MaxConcernFamilies:                3,
		MinTestToProductionRatioPercent:   20,
	}
}

func baseDefaultProductionRisk() core.ProductionRiskConfig {
	return core.ProductionRiskConfig{
		Enabled:           boolPtr(true),
		WarnThreshold:     35,
		FailThreshold:     70,
		ReliabilityWeight: 12,
		DataWeight:        15,
		FailWeight:        25,
		WarnWeight:        10,
	}
}

func baseDefaultContractRules() core.ContractRulesConfig {
	return core.ContractRulesConfig{
		GoExportedBreaking:   boolPtr(true),
		CPPPublicBreaking:    boolPtr(true),
		OpenAPIBreaking:      boolPtr(true),
		ProtoBreaking:        boolPtr(true),
		MigrationDestructive: boolPtr(true),
		MigrationPaths:       []string{"migrations/", "db/migrate/", "alembic/"},
	}
}

func baseDefaultCacheConfig() core.CacheConfig {
	return core.CacheConfig{
		Enabled: boolPtr(true),
		Path:    ".codeguard/cache.json",
	}
}
