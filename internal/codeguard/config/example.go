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
			ContractRules: core.ContractRulesConfig{
				GoExportedBreaking:   boolPtr(true),
				OpenAPIBreaking:      boolPtr(true),
				ProtoBreaking:        boolPtr(true),
				MigrationDestructive: boolPtr(true),
				MigrationPaths:       []string{"migrations/", "db/migrate/", "alembic/"},
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
