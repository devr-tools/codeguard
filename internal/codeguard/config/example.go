package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

func baseExampleConfig() core.Config {
	return core.Config{
		Name: "codeguard-default",
		Targets: []core.TargetConfig{{
			Name:        "repository",
			Path:        ".",
			Language:    "go",
			Entrypoints: []string{"cmd/codeguard"},
		}},
		Checks: core.CheckConfig{
			Quality:  true,
			Design:   true,
			Security: true,
			Prompts:  true,
			CI:       true,
			QualityRules: core.QualityRulesConfig{
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
			},
			DesignRules: core.DesignRulesConfig{
				RequireCmdThroughInternalCLI: boolPtr(true),
				ForbidInternalImportCmd:      boolPtr(true),
				ForbidServiceImportInternal:  boolPtr(true),
				ForbidServiceImportCmd:       boolPtr(true),
				MaxDeclsPerFile:              12,
				MaxMethodsPerType:            8,
				MaxInterfaceMethods:          5,
				ForbiddenPackageNames:        []string{"util", "utils", "common", "helpers", "misc"},
			},
			PromptRules: core.PromptRulesConfig{
				FileExtensions:            []string{".prompt", ".md", ".txt", ".tmpl", ".yaml", ".yml", ".json"},
				PathContains:              []string{"prompt", "system", "instruction", "template"},
				ForbidSecretInterpolation: boolPtr(true),
				ForbidUnsafeInstructions:  boolPtr(true),
			},
			CIRules: core.CIRulesConfig{
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
			},
			SecurityRules: core.SecurityRulesConfig{
				GovulncheckMode:    "auto",
				GovulncheckCommand: "govulncheck",
			},
		},
		AI: core.AIConfig{
			Enabled: boolPtr(false),
			Provider: core.AIProviderConfig{
				Type:      "openai",
				Model:     "gpt-5",
				BaseURL:   "https://api.openai.com/v1",
				APIKeyEnv: "OPENAI_API_KEY",
			},
			Cache: core.AICacheConfig{
				Path: ".codeguard/ai-cache.json",
			},
			HybridTriage: core.AIHybridTriageConfig{
				Enabled:             boolPtr(true),
				SuppressDismissed:   boolPtr(true),
				CandidateSections:   []string{"Code Quality", "Design Patterns", "Security", "Custom Rules"},
				CandidateSeverities: []string{"warn", "fail"},
			},
			Semantic: core.AISemanticConfig{
				Enabled:                 boolPtr(true),
				FunctionContract:        boolPtr(true),
				MisleadingErrorMessages: boolPtr(true),
				TestBehaviorCoverage:    boolPtr(true),
			},
			AutoFix: core.AIAutoFixConfig{
				Enabled:     boolPtr(false),
				VerifyTests: boolPtr(true),
				MaxFixes:    5,
			},
		},
		Output: core.OutputConfig{Format: "text"},
		Cache: core.CacheConfig{
			Enabled: boolPtr(true),
			Path:    ".codeguard/cache.json",
		},
	}
}

func boolPtr(v bool) *bool {
	return &v
}
