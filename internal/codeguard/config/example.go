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
		Quality:          true,
		Design:           true,
		Security:         true,
		Prompts:          true,
		CI:               true,
		SupplyChain:      false,
		QualityRules:     exampleQualityRules(),
		DesignRules:      exampleDesignRules(),
		PromptRules:      examplePromptRules(),
		CIRules:          exampleCIRules(),
		SecurityRules:    exampleSecurityRules(),
		SupplyChainRules: exampleSupplyChainRules(),
		ContractRules:    exampleContractRules(),
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
