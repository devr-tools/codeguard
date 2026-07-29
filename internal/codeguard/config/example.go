package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

func baseExampleConfig() core.Config {
	return core.Config{
		Name:    "codeguard-default",
		Targets: exampleTargets(),
		Checks:  exampleChecks(),
		AI:      exampleAIConfig(),
		Output:  core.OutputConfig{Format: "text"},
		Cache:   exampleCacheConfig(),
		Parsers: core.ParsersConfig{TreeSitter: core.TreeSitterModeOff},
	}
}

func exampleTargets() []core.TargetConfig {
	return []core.TargetConfig{{
		Name:        "repository",
		Path:        ".",
		Language:    "go",
		Entrypoints: []string{"cmd/codeguard"},
	}}
}

func exampleChecks() core.CheckConfig {
	return core.CheckConfig{
		Quality: true,
		Design:  true,
		// Performance is opt-in while the rules settle into their own section;
		// they previously ran (enabled) inside quality under quality.* ids. The
		// explicit false (vs nil) writes the key into generated configs so new
		// users discover it, and suppresses the upgrade hint in scan output.
		Performance:        boolPtr(false),
		Security:           true,
		Prompts:            true,
		CI:                 true,
		SupplyChain:        false,
		Delivery:           boolPtr(false),
		Reliability:        boolPtr(false),
		Data:               boolPtr(false),
		Observability:      boolPtr(false),
		Operations:         boolPtr(false),
		Change:             boolPtr(false),
		QualityRules:       exampleQualityRules(),
		PerformanceRules:   examplePerformanceRules(),
		DesignRules:        exampleDesignRules(),
		PromptRules:        examplePromptRules(),
		CIRules:            exampleCIRules(),
		DeliveryRules:      exampleDeliveryRules(),
		SecurityRules:      exampleSecurityRules(),
		SupplyChainRules:   exampleSupplyChainRules(),
		ReliabilityRules:   exampleReliabilityRules(),
		DataRules:          exampleDataRules(),
		ObservabilityRules: exampleObservabilityRules(),
		OperationsRules:    exampleOperationsRules(),
		ChangeRules:        exampleChangeRules(),
		ContractRules:      exampleContractRules(),
		ContextRules:       exampleContextRules(),
		ProductionRisk:     exampleProductionRisk(),
	}
}

func exampleContextRules() core.ContextRulesConfig {
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

func exampleSupplyChainRules() core.SupplyChainRulesConfig {
	return core.SupplyChainRulesConfig{
		RequireLockfile:     boolPtr(true),
		DetectLockfileDrift: boolPtr(true),
		DetectUnpinned:      boolPtr(true),
		DetectProvenance:    boolPtr(true),
	}
}

func exampleDeliveryRules() core.DeliveryRulesConfig {
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

func exampleObservabilityRules() core.ObservabilityRulesConfig {
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

func exampleOperationsRules() core.OperationsRulesConfig {
	return core.OperationsRulesConfig{
		DetectMissingOwner:   boolPtr(true),
		DetectMissingRunbook: boolPtr(true),
		OwnerFilePatterns:    []string{"CODEOWNERS", ".github/CODEOWNERS", "OWNERS", "owners.yaml", "catalog-info.yaml", "service.yaml", "service.yml"},
		RunbookPathPatterns:  []string{"runbook", "runbooks", "docs/runbooks", "ops", "operations"},
		CriticalPathPatterns: []string{"cmd/", "internal/", "service", "api", "worker", "consumer", "job", "payment", "auth", "deploy", "migrations"},
	}
}

func exampleReliabilityRules() core.ReliabilityRulesConfig {
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

func exampleDataRules() core.DataRulesConfig {
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

func exampleChangeRules() core.ChangeRulesConfig {
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

func exampleProductionRisk() core.ProductionRiskConfig {
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

func exampleContractRules() core.ContractRulesConfig {
	return core.ContractRulesConfig{
		GoExportedBreaking:   boolPtr(true),
		CPPPublicBreaking:    boolPtr(true),
		OpenAPIBreaking:      boolPtr(true),
		ProtoBreaking:        boolPtr(true),
		MigrationDestructive: boolPtr(true),
		MigrationPaths:       []string{"migrations/", "db/migrate/", "alembic/"},
	}
}

func exampleCacheConfig() core.CacheConfig {
	return core.CacheConfig{
		Enabled: boolPtr(true),
		Path:    ".codeguard/cache.json",
	}
}
