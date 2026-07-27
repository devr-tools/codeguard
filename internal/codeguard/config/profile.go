package config

import (
	"fmt"
	"sort"
	"strconv"
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
			cfg.Checks.QualityRules.CloneTokenThreshold = 120
			cfg.Checks.DesignRules.MaxDeclsPerFile = 16
			cfg.Checks.DesignRules.MaxMethodsPerType = 10
			cfg.Checks.DesignRules.MaxInterfaceMethods = 8
			cfg.Checks.CIRules.RequiredReleaseFiles = nil
			cfg.Checks.SecurityRules.GovulncheckMode = "auto"
		},
	},
	"strict": {
		description: "Tighter quality, design, and security thresholds for hard gates.",
		apply:       applyStrictProfile,
	},
	"enterprise": {
		description: "Strict gates with release and automation policy suitable for regulated delivery.",
		apply: func(cfg *core.Config) {
			applyStrictProfile(cfg)
			cfg.Checks.CIRules.RequiredReleaseFiles = []string{".goreleaser.yaml"}
			cfg.Checks.CIRules.RequiredAutomationPaths = []string{"Makefile", ".github/workflows/ci.yml"}
			cfg.Checks.Data = boolPtr(true)
			cfg.Checks.Change = boolPtr(true)
			cfg.Checks.Observability = boolPtr(true)
			cfg.Checks.Operations = boolPtr(true)
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
			cfg.Checks.QualityRules.CloneTokenThreshold = 75
			cfg.Checks.QualityRules.AIProvenance.Enabled = boolPtr(true)
			cfg.Checks.QualityRules.AIProvenance.SlopScoreWarnThreshold = 10
			cfg.Checks.QualityRules.AIProvenance.SlopScoreFailThreshold = 25
			cfg.Checks.Reliability = boolPtr(true)
			cfg.Checks.Data = boolPtr(true)
			cfg.Checks.Change = boolPtr(true)
			cfg.Checks.Observability = boolPtr(true)
			cfg.Checks.ChangeRules.MaxChangedFiles = 20
			cfg.Checks.ChangeRules.MaxChangedDirectories = 6
			cfg.Checks.ChangeRules.MaxChangedLines = 600
			cfg.Checks.ChangeRules.MinTestToProductionRatioPercent = 30
		},
	},
}

func applyStrictProfile(cfg *core.Config) {
	cfg.Checks.QualityRules.MaxFileLines = 300
	cfg.Checks.QualityRules.MaxFunctionLines = 60
	cfg.Checks.QualityRules.MaxParameters = 4
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 8
	cfg.Checks.QualityRules.CloneTokenThreshold = 60
	cfg.Checks.DesignRules.MaxDeclsPerFile = 10
	cfg.Checks.DesignRules.MaxMethodsPerType = 6
	cfg.Checks.DesignRules.MaxInterfaceMethods = 4
	cfg.Checks.SecurityRules.GovulncheckMode = "required"
	cfg.Checks.Contracts = boolPtr(true)
	cfg.Checks.Reliability = boolPtr(true)
	cfg.Checks.Change = boolPtr(true)
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

// RenderPolicyProfileComparison renders the profile comparison section for
// documentation from the active profile definitions. Keeping this output
// derived from profile data prevents documentation thresholds from drifting.
func RenderPolicyProfileComparison() string {
	profiles := []struct {
		label string
		name  string
	}{
		{label: "Baseline"},
		{label: "Startup", name: "startup"},
		{label: "Strict", name: "strict"},
		{label: "Enterprise", name: "enterprise"},
		{label: "AI-safe", name: "ai-safe"},
	}

	configs := make([]core.Config, len(profiles))
	configs[0] = ExampleConfig()
	for i := 1; i < len(profiles); i++ {
		configs[i], _ = ExampleConfigForProfile(profiles[i].name)
	}

	var b strings.Builder
	b.WriteString("<!-- BEGIN GENERATED: policy-profile-comparison -->\n")
	b.WriteString("| Setting")
	for _, profile := range profiles {
		b.WriteString(" | ")
		b.WriteString(profile.label)
	}
	b.WriteString(" |\n")
	b.WriteString("| ---")
	for range profiles {
		b.WriteString(" | ---:")
	}
	b.WriteString(" |\n")
	writeProfileComparisonRow(&b, "`quality_rules.max_file_lines`", configs, func(cfg core.Config) string {
		return strconv.Itoa(cfg.Checks.QualityRules.MaxFileLines)
	})
	writeProfileComparisonRow(&b, "`quality_rules.max_function_lines`", configs, func(cfg core.Config) string {
		return strconv.Itoa(cfg.Checks.QualityRules.MaxFunctionLines)
	})
	writeProfileComparisonRow(&b, "`quality_rules.max_parameters`", configs, func(cfg core.Config) string {
		return strconv.Itoa(cfg.Checks.QualityRules.MaxParameters)
	})
	writeProfileComparisonRow(&b, "`quality_rules.max_cyclomatic_complexity`", configs, func(cfg core.Config) string {
		return strconv.Itoa(cfg.Checks.QualityRules.MaxCyclomaticComplexity)
	})
	writeProfileComparisonRow(&b, "`quality_rules.clone_token_threshold`", configs, func(cfg core.Config) string {
		return strconv.Itoa(cfg.Checks.QualityRules.CloneTokenThreshold)
	})
	writeProfileComparisonRow(&b, "`design_rules.max_decls_per_file`", configs, func(cfg core.Config) string {
		return strconv.Itoa(cfg.Checks.DesignRules.MaxDeclsPerFile)
	})
	writeProfileComparisonRow(&b, "`design_rules.max_methods_per_type`", configs, func(cfg core.Config) string {
		return strconv.Itoa(cfg.Checks.DesignRules.MaxMethodsPerType)
	})
	writeProfileComparisonRow(&b, "`design_rules.max_interface_methods`", configs, func(cfg core.Config) string {
		return strconv.Itoa(cfg.Checks.DesignRules.MaxInterfaceMethods)
	})
	writeProfileComparisonRow(&b, "`security_rules.govulncheck_mode`", configs, func(cfg core.Config) string {
		return cfg.Checks.SecurityRules.GovulncheckMode
	})
	writeProfileComparisonRow(&b, "`ci_rules.required_release_files`", configs, func(cfg core.Config) string {
		return profileStringSlice(cfg.Checks.CIRules.RequiredReleaseFiles)
	})
	writeProfileComparisonRow(&b, "`ci_rules.required_automation_paths`", configs, func(cfg core.Config) string {
		return profileStringSlice(cfg.Checks.CIRules.RequiredAutomationPaths)
	})
	writeProfileComparisonRow(&b, "`contracts`", configs, func(cfg core.Config) string {
		if cfg.Checks.Contracts == nil {
			return "scan-mode"
		}
		return strconv.FormatBool(*cfg.Checks.Contracts)
	})
	writeProfileComparisonRow(&b, "`reliability`", configs, func(cfg core.Config) string {
		if cfg.Checks.Reliability == nil {
			return "scan-mode"
		}
		return strconv.FormatBool(*cfg.Checks.Reliability)
	})
	writeProfileComparisonRow(&b, "`data`", configs, func(cfg core.Config) string {
		if cfg.Checks.Data == nil {
			return "scan-mode"
		}
		return strconv.FormatBool(*cfg.Checks.Data)
	})
	writeProfileComparisonRow(&b, "`observability`", configs, func(cfg core.Config) string {
		if cfg.Checks.Observability == nil {
			return "scan-mode"
		}
		return strconv.FormatBool(*cfg.Checks.Observability)
	})
	writeProfileComparisonRow(&b, "`operations`", configs, func(cfg core.Config) string {
		if cfg.Checks.Operations == nil {
			return "scan-mode"
		}
		return strconv.FormatBool(*cfg.Checks.Operations)
	})
	writeProfileComparisonRow(&b, "`change`", configs, func(cfg core.Config) string {
		if cfg.Checks.Change == nil {
			return "scan-mode"
		}
		return strconv.FormatBool(*cfg.Checks.Change)
	})
	writeProfileComparisonRow(&b, "`change_rules.max_changed_files`", configs, func(cfg core.Config) string {
		return strconv.Itoa(cfg.Checks.ChangeRules.MaxChangedFiles)
	})
	writeProfileComparisonRow(&b, "`change_rules.max_changed_directories`", configs, func(cfg core.Config) string {
		return strconv.Itoa(cfg.Checks.ChangeRules.MaxChangedDirectories)
	})
	writeProfileComparisonRow(&b, "`change_rules.max_changed_lines`", configs, func(cfg core.Config) string {
		return strconv.Itoa(cfg.Checks.ChangeRules.MaxChangedLines)
	})
	writeProfileComparisonRow(&b, "`change_rules.min_test_to_production_ratio_percent`", configs, func(cfg core.Config) string {
		return strconv.Itoa(cfg.Checks.ChangeRules.MinTestToProductionRatioPercent)
	})
	b.WriteString("<!-- END GENERATED: policy-profile-comparison -->\n")
	return b.String()
}

func writeProfileComparisonRow(b *strings.Builder, setting string, configs []core.Config, value func(core.Config) string) {
	b.WriteString("| ")
	b.WriteString(setting)
	for _, cfg := range configs {
		b.WriteString(" | ")
		b.WriteString(value(cfg))
	}
	b.WriteString(" |\n")
}

func profileStringSlice(values []string) string {
	if len(values) == 0 {
		return "—"
	}
	return strings.Join(values, "<br>")
}

func normalizeProfile(profile string) string {
	return strings.ToLower(strings.TrimSpace(profile))
}
