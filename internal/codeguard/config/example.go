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
		Performance:      boolPtr(false),
		Security:         true,
		Prompts:          true,
		CI:               true,
		SupplyChain:      false,
		QualityRules:     exampleQualityRules(),
		PerformanceRules: examplePerformanceRules(),
		DesignRules:      exampleDesignRules(),
		PromptRules:      examplePromptRules(),
		CIRules:          exampleCIRules(),
		SecurityRules:    exampleSecurityRules(),
		SupplyChainRules: exampleSupplyChainRules(),
		ContractRules:    exampleContractRules(),
		ContextRules:     exampleContextRules(),
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
	}
}

func exampleContractRules() core.ContractRulesConfig {
	return core.ContractRulesConfig{
		GoExportedBreaking:   boolPtr(true),
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
