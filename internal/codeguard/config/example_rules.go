package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

func exampleQualityRules() core.QualityRulesConfig {
	return core.QualityRulesConfig{
		MaxFileLines:            400,
		MaxFunctionLines:        80,
		MaxParameters:           5,
		MaxCyclomaticComplexity: 10,
		CloneTokenThreshold:     60,
		AIProvenance: core.AIProvenanceConfig{
			Enabled:                boolPtr(true),
			EnvVars:                []string{"CODEGUARD_AI_ASSISTED"},
			CommitTrailers:         []string{"AI-Assisted", "AI-Generated"},
			SlopScoreWarnThreshold: 20,
			SlopScoreFailThreshold: 40,
		},
	}
}

func exampleDesignRules() core.DesignRulesConfig {
	return core.DesignRulesConfig{
		RequireCmdThroughInternalCLI: boolPtr(true),
		ForbidInternalImportCmd:      boolPtr(true),
		ForbidServiceImportInternal:  boolPtr(true),
		ForbidServiceImportCmd:       boolPtr(true),
		MaxDeclsPerFile:              12,
		GodModuleThreshold:           25,
		HighImpactChangeThreshold:    10,
		MaxMethodsPerType:            8,
		MaxInterfaceMethods:          5,
		ForbiddenPackageNames:        []string{"util", "utils", "common", "helpers", "misc"},
	}
}

func examplePromptRules() core.PromptRulesConfig {
	return core.PromptRulesConfig{
		FileExtensions:            []string{".prompt", ".md", ".txt", ".tmpl", ".yaml", ".yml", ".json"},
		PathContains:              []string{"prompt", "system", "instruction", "template"},
		ForbidSecretInterpolation: boolPtr(true),
		ForbidUnsafeInstructions:  boolPtr(true),
	}
}

func exampleCIRules() core.CIRulesConfig {
	return core.CIRulesConfig{
		RequireWorkflowDir: boolPtr(true),
		RequiredWorkflowFiles: []string{
			".github/workflows/ci.yml",
		},
		WorkflowContentRules: []core.WorkflowRuleConfig{{
			Path:             ".github/workflows/ci.yml",
			RequiredContains: []string{"actions/checkout", "go test ./..."},
		}},
		RequiredReleaseFiles:    []string{".goreleaser.yaml"},
		RequiredAutomationPaths: []string{"Makefile"},
		AllowedTestPaths:        []string{"tests/**"},
	}
}

func exampleSecurityRules() core.SecurityRulesConfig {
	return core.SecurityRulesConfig{
		GovulncheckMode:         "auto",
		GovulncheckCommand:      "govulncheck",
		TypeScriptTaintMaxDepth: 8,
	}
}
