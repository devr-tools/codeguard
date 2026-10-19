package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

func baseDefaultQualityRules() core.QualityRulesConfig {
	return core.QualityRulesConfig{
		MaxFileLines:            400,
		MaxFunctionLines:        80,
		MaxParameters:           5,
		MaxCyclomaticComplexity: 10,
		CloneTokenThreshold:     90,
		AIProvenance: core.AIProvenanceConfig{
			Enabled:                boolPtr(true),
			EnvVars:                []string{"CODEGUARD_AI_ASSISTED"},
			CommitTrailers:         []string{"AI-Assisted", "AI-Generated"},
			SlopScoreWarnThreshold: 20,
			SlopScoreFailThreshold: 40,
		},
		AIChangeRisk: core.AIChangeRiskConfig{
			Enabled:       boolPtr(true),
			WarnThreshold: 30,
			FailThreshold: 60,
		},
		CPPTooling: core.CPPToolingConfig{
			ClangFormatMode:    core.ExternalToolModeOff,
			ClangFormatCommand: "clang-format",
			CompilerMode:       core.ExternalToolModeOff,
			CompilerCommand:    "clang++",
		},
		Naming: core.QualityNamingConfig{
			AllowedAbbreviations:    []string{"api", "cli", "cpp", "db", "html", "http", "https", "id", "io", "ip", "js", "json", "orm", "rpc", "sdk", "sql", "tcp", "ts", "ui", "url", "uuid", "xml", "yaml"},
			RoleSuffixWarnThreshold: 4,
		},
	}
}

func baseDefaultPerformanceRules() core.PerformanceRulesConfig {
	return core.PerformanceRulesConfig{
		DetectPreallocInLoop: boolPtr(false),
	}
}

func baseDefaultDesignRules() core.DesignRulesConfig {
	return core.DesignRulesConfig{
		// Package-layering policy: off by default. These four rules encode one
		// specific repository layout (cmd/ delegating to internal/cli, a pkg/
		// facade that must not reach into internal/ or cmd/) and report at
		// fail level. `design.cmd-through-internal-cli` additionally only ever
		// matches an import of ".../pkg/codeguard", so it cannot fire outside
		// this repository at all. A repository that wants the layering
		// enforced states it in design_rules; the starter config and the
		// self-scan config both do.
		RequireCmdThroughInternalCLI: boolPtr(false),
		ForbidInternalImportCmd:      boolPtr(false),
		ForbidServiceImportInternal:  boolPtr(false),
		ForbidServiceImportCmd:       boolPtr(false),
		MaxDeclsPerFile:              12,
		GodModuleThreshold:           25,
		HighImpactChangeThreshold:    10,
		MaxMethodsPerType:            8,
		MaxInterfaceMethods:          5,
		ForbiddenPackageNames:        []string{"util", "utils", "common", "helpers", "misc"},
	}
}

func baseDefaultPromptRules() core.PromptRulesConfig {
	return core.PromptRulesConfig{
		FileExtensions:            []string{".prompt", ".md", ".txt", ".tmpl", ".yaml", ".yml", ".json"},
		PathContains:              []string{"prompt", "system", "instruction", "template"},
		ForbidSecretInterpolation: boolPtr(true),
		ForbidUnsafeInstructions:  boolPtr(true),
	}
}

// baseDefaultCIRules is deliberately almost empty. Every list in
// CIRulesConfig names a repository convention — a workflow filename, a release
// artifact, a test directory, a literal command string — that codeguard cannot
// know applies. Each rule already treats an empty list as "not configured", so
// leaving them unset keeps the mechanism intact while enforcing nothing:
//
//   - required_workflow_files / required_release_files /
//     required_automation_paths: requiredPathFindings iterates the list.
//   - workflow_content_rules: workflowContentFindings iterates the list.
//   - required_gates: missingRequiredGateFindings returns early when empty.
//   - allowed_test_paths: testFileLocationFindings returns early when empty.
//     A non-empty default here hard-failed every repository with co-located
//     tests (idiomatic Go `_test.go`, `*.spec.ts`, pytest `test_*.py`).
//
// The opinionated versions of all of these live in the starter config and in
// the enterprise/strict profiles.
func baseDefaultCIRules() core.CIRulesConfig {
	return core.CIRulesConfig{
		// The only CI default: enabling the section is itself the assertion
		// that the repository wants pipeline policy checked, and every other
		// CI rule reads workflow files from .github/workflows.
		RequireWorkflowDir: boolPtr(true),
	}
}

func baseDefaultSecurityRules() core.SecurityRulesConfig {
	return core.SecurityRulesConfig{
		GovulncheckMode:         "auto",
		GovulncheckCommand:      "govulncheck",
		TypeScriptTaintMaxDepth: 8,
		Secrets: &core.SecretsRulesConfig{
			Enabled:    boolPtr(true),
			AllowPaths: []string{"testdata/**", "**/testdata/**"},
		},
	}
}
