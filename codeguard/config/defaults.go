package config

import "github.com/devr-tools/codeguard/codeguard/core"

func boolPtr(value bool) *bool {
	return &value
}

func ExampleConfig() core.Config {
	return core.Config{
		Name: "codeguard-default",
		Targets: []core.TargetConfig{
			{
				Name:        "repository",
				Path:        ".",
				Language:    "go",
				Entrypoints: []string{"cmd/codeguard"},
			},
		},
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
				RequireWorkflowDir:    boolPtr(true),
				RequiredWorkflowFiles: []string{".github/workflows/ci.yml", ".github/workflows/cd.yml", ".github/workflows/release.yml"},
				WorkflowContentRules: []core.WorkflowRuleConfig{
					{
						Path:             ".github/workflows/ci.yml",
						RequiredContains: []string{"actions/checkout", "go test ./..."},
					},
					{
						Path:             ".github/workflows/cd.yml",
						RequiredContains: []string{"googleapis/release-please-action", "uses: ./.github/workflows/release.yml", "RELEASE_PLEASE_TOKEN"},
					},
					{
						Path:             ".github/workflows/release.yml",
						RequiredContains: []string{"goreleaser/goreleaser-action@v7", "sync-homebrew-formula", "Formula/codeguard.rb"},
					},
				},
				RequiredReleaseFiles:    []string{".goreleaser.yaml", "Dockerfile.release", ".github/release-please-config.json", ".release-please-manifest.json", "CHANGELOG.md"},
				RequiredAutomationPaths: []string{"Makefile", "scripts/commit.sh"},
			},
			SecurityRules: core.SecurityRulesConfig{
				GovulncheckMode:    "auto",
				GovulncheckCommand: "govulncheck",
			},
		},
		Output: core.OutputConfig{
			Format: "text",
		},
	}
}
