package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type profileSpec struct {
	description string
	apply       func(*core.Config)
}

var profileCatalog = map[string]profileSpec{
	"startup": {
		description: "Looser thresholds for fast-moving repos with lightweight release policy.",
		apply: func(cfg *core.Config) {
			cfg.Checks.QualityRules.MaxFileLines = 600
			cfg.Checks.QualityRules.MaxFunctionLines = 120
			cfg.Checks.QualityRules.MaxParameters = 7
			cfg.Checks.QualityRules.MaxCyclomaticComplexity = 15
			cfg.Checks.QualityRules.CloneTokenThreshold = 90
			cfg.Checks.DesignRules.MaxDeclsPerFile = 16
			cfg.Checks.DesignRules.MaxMethodsPerType = 10
			cfg.Checks.DesignRules.MaxInterfaceMethods = 8
			cfg.Checks.CIRules.RequiredReleaseFiles = nil
			cfg.Checks.SecurityRules.GovulncheckMode = "auto"
		},
	},
	"strict": {
		description: "Tighter quality, design, and security thresholds for hard gates.",
		apply: func(cfg *core.Config) {
			cfg.Checks.QualityRules.MaxFileLines = 300
			cfg.Checks.QualityRules.MaxFunctionLines = 60
			cfg.Checks.QualityRules.MaxParameters = 4
			cfg.Checks.QualityRules.MaxCyclomaticComplexity = 8
			cfg.Checks.QualityRules.CloneTokenThreshold = 45
			cfg.Checks.DesignRules.MaxDeclsPerFile = 10
			cfg.Checks.DesignRules.MaxMethodsPerType = 6
			cfg.Checks.DesignRules.MaxInterfaceMethods = 4
			cfg.Checks.SecurityRules.GovulncheckMode = "required"
		},
	},
	"enterprise": {
		description: "Strict gates with release and automation policy suitable for regulated delivery.",
		apply: func(cfg *core.Config) {
			cfg.Checks.QualityRules.MaxFileLines = 300
			cfg.Checks.QualityRules.MaxFunctionLines = 60
			cfg.Checks.QualityRules.MaxParameters = 4
			cfg.Checks.QualityRules.MaxCyclomaticComplexity = 8
			cfg.Checks.QualityRules.CloneTokenThreshold = 45
			cfg.Checks.DesignRules.MaxDeclsPerFile = 10
			cfg.Checks.DesignRules.MaxMethodsPerType = 6
			cfg.Checks.DesignRules.MaxInterfaceMethods = 4
			cfg.Checks.SecurityRules.GovulncheckMode = "required"
			cfg.Checks.CIRules.RequiredReleaseFiles = []string{".goreleaser.yaml"}
			cfg.Checks.CIRules.RequiredAutomationPaths = []string{"Makefile", ".github/workflows/ci.yml"}
		},
	},
	"ai-safe": {
		description: "Bias toward prompt governance and dependency hygiene for AI-heavy repositories.",
		apply: func(cfg *core.Config) {
			cfg.Checks.Prompts = true
			cfg.Checks.Security = true
			cfg.Checks.PromptRules.PathContains = []string{"prompt", "system", "instruction", "template", "agent", "policy"}
			cfg.Checks.PromptRules.FileExtensions = []string{".prompt", ".md", ".mdx", ".txt", ".tmpl", ".yaml", ".yml", ".json"}
			cfg.Checks.SecurityRules.GovulncheckMode = "required"
			cfg.Checks.QualityRules.MaxFunctionLines = 70
			cfg.Checks.QualityRules.MaxCyclomaticComplexity = 9
			cfg.Checks.QualityRules.CloneTokenThreshold = 50
			cfg.Checks.QualityRules.AIProvenance.Enabled = boolPtr(true)
			cfg.Checks.QualityRules.AIProvenance.SlopScoreWarnThreshold = 10
			cfg.Checks.QualityRules.AIProvenance.SlopScoreFailThreshold = 25
		},
	},
}

func ExampleConfig() core.Config {
	return baseExampleConfig()
}

func ExampleConfigForProfile(profile string) (core.Config, error) {
	cfg := baseExampleConfig()
	normalized := normalizeProfile(profile)
	if normalized == "" {
		return cfg, nil
	}

	spec, ok := profileCatalog[normalized]
	if !ok {
		return core.Config{}, fmt.Errorf("unknown profile %q", profile)
	}
	spec.apply(&cfg)
	cfg.Profile = normalized
	return cfg, nil
}

func ProfileList() []core.PolicyProfile {
	names := make([]string, 0, len(profileCatalog))
	for name := range profileCatalog {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]core.PolicyProfile, 0, len(names))
	for _, name := range names {
		out = append(out, core.PolicyProfile{
			Name:        name,
			Description: profileCatalog[name].description,
		})
	}
	return out
}

func normalizeProfile(profile string) string {
	return strings.ToLower(strings.TrimSpace(profile))
}
