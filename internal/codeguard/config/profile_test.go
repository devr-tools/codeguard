package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type profileThresholds struct {
	maxFileLines            int
	maxFunctionLines        int
	maxParameters           int
	maxCyclomaticComplexity int
	cloneTokenThreshold     int
	maxDeclsPerFile         int
	maxMethodsPerType       int
	maxInterfaceMethods     int
	govulncheckMode         string
	requiredReleaseFiles    []string
	requiredAutomationPaths []string
	contracts               *bool
	reliability             *bool
	data                    *bool
	observability           *bool
	operations              *bool
	delivery                *bool
	change                  *bool
	maxChangedFiles         int
	maxChangedDirectories   int
	maxChangedLines         int
	minTestProdRatioPercent int
}

func TestProfilesPreserveExpectedPolicyValues(t *testing.T) {
	for name, want := range expectedProfileThresholds() {
		t.Run(name, func(t *testing.T) {
			var (
				cfg = ExampleConfig()
				err error
			)
			if name != "baseline" {
				cfg, err = ExampleConfigForProfile(name)
				if err != nil {
					t.Fatalf("ExampleConfigForProfile(%q) error = %v", name, err)
				}
			}

			got := profileThresholdsFromConfig(cfg)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("profile policy = %#v, want %#v", got, want)
			}
		})
	}
}

func TestReviewProfilesEnableLocalPrecisionAndRegressionSignals(t *testing.T) {
	aiSafe, err := ExampleConfigForProfile("ai-safe")
	if err != nil {
		t.Fatalf("ExampleConfigForProfile(ai-safe) error = %v", err)
	}
	if aiSafe.Checks.QualityRules.LocalPrecision != nil && !*aiSafe.Checks.QualityRules.LocalPrecision {
		t.Fatal("ai-safe profile must not disable local quality precision checks")
	}
	if aiSafe.Checks.Change == nil || !*aiSafe.Checks.Change {
		t.Fatal("ai-safe profile should enable change-safety regression checks")
	}
	if aiSafe.Checks.Reliability == nil || !*aiSafe.Checks.Reliability {
		t.Fatal("ai-safe profile should enable reliability checks")
	}

	strict, err := ExampleConfigForProfile("strict")
	if err != nil {
		t.Fatalf("ExampleConfigForProfile(strict) error = %v", err)
	}
	if strict.Checks.Change == nil || !*strict.Checks.Change {
		t.Fatal("strict profile should enable change-safety regression checks")
	}
	if strict.Checks.Data == nil || *strict.Checks.Data {
		t.Fatal("strict profile should focus regressions without enabling data-correctness production-readiness checks")
	}
	if strict.Checks.Observability == nil || *strict.Checks.Observability {
		t.Fatal("strict profile should focus regressions without enabling observability production-readiness checks")
	}
}

func expectedProfileThresholds() map[string]profileThresholds {
	profiles := baselineAndStartupProfileThresholds()
	for name, thresholds := range strictProfileThresholds() {
		profiles[name] = thresholds
	}
	return profiles
}

func baselineAndStartupProfileThresholds() map[string]profileThresholds {
	return map[string]profileThresholds{
		"baseline": {
			maxFileLines:            400,
			maxFunctionLines:        80,
			maxParameters:           5,
			maxCyclomaticComplexity: 10,
			cloneTokenThreshold:     90,
			maxDeclsPerFile:         12,
			maxMethodsPerType:       8,
			maxInterfaceMethods:     5,
			govulncheckMode:         "auto",
			requiredReleaseFiles:    []string{".goreleaser.yaml"},
			requiredAutomationPaths: []string{"Makefile"},
			reliability:             boolPtr(false),
			data:                    boolPtr(false),
			observability:           boolPtr(false),
			operations:              boolPtr(false),
			delivery:                boolPtr(false),
			change:                  boolPtr(false),
			maxChangedFiles:         25,
			maxChangedDirectories:   8,
			maxChangedLines:         800,
			minTestProdRatioPercent: 20,
		},
		"startup": {
			maxFileLines:            600,
			maxFunctionLines:        120,
			maxParameters:           7,
			maxCyclomaticComplexity: 15,
			cloneTokenThreshold:     120,
			maxDeclsPerFile:         16,
			maxMethodsPerType:       10,
			maxInterfaceMethods:     8,
			govulncheckMode:         "auto",
			requiredAutomationPaths: []string{"Makefile"},
			reliability:             boolPtr(false),
			data:                    boolPtr(false),
			observability:           boolPtr(false),
			operations:              boolPtr(false),
			delivery:                boolPtr(false),
			change:                  boolPtr(false),
			maxChangedFiles:         25,
			maxChangedDirectories:   8,
			maxChangedLines:         800,
			minTestProdRatioPercent: 20,
		},
	}
}

func strictProfileThresholds() map[string]profileThresholds {
	return map[string]profileThresholds{
		"strict": {
			maxFileLines:            300,
			maxFunctionLines:        60,
			maxParameters:           4,
			maxCyclomaticComplexity: 8,
			cloneTokenThreshold:     60,
			maxDeclsPerFile:         10,
			maxMethodsPerType:       6,
			maxInterfaceMethods:     4,
			govulncheckMode:         "required",
			requiredReleaseFiles:    []string{".goreleaser.yaml"},
			requiredAutomationPaths: []string{"Makefile"},
			contracts:               boolPtr(true),
			reliability:             boolPtr(true),
			data:                    boolPtr(false),
			observability:           boolPtr(false),
			operations:              boolPtr(false),
			delivery:                boolPtr(false),
			change:                  boolPtr(true),
			maxChangedFiles:         25,
			maxChangedDirectories:   8,
			maxChangedLines:         800,
			minTestProdRatioPercent: 20,
		},
		"enterprise": {
			maxFileLines:            300,
			maxFunctionLines:        60,
			maxParameters:           4,
			maxCyclomaticComplexity: 8,
			cloneTokenThreshold:     60,
			maxDeclsPerFile:         10,
			maxMethodsPerType:       6,
			maxInterfaceMethods:     4,
			govulncheckMode:         "required",
			requiredReleaseFiles:    []string{".goreleaser.yaml"},
			requiredAutomationPaths: []string{"Makefile", ".github/workflows/ci.yml"},
			contracts:               boolPtr(true),
			reliability:             boolPtr(true),
			data:                    boolPtr(true),
			observability:           boolPtr(true),
			operations:              boolPtr(true),
			delivery:                boolPtr(true),
			change:                  boolPtr(true),
			maxChangedFiles:         25,
			maxChangedDirectories:   8,
			maxChangedLines:         800,
			minTestProdRatioPercent: 20,
		},
		"ai-safe": {
			maxFileLines:            400,
			maxFunctionLines:        70,
			maxParameters:           5,
			maxCyclomaticComplexity: 9,
			cloneTokenThreshold:     75,
			maxDeclsPerFile:         12,
			maxMethodsPerType:       8,
			maxInterfaceMethods:     5,
			govulncheckMode:         "required",
			requiredReleaseFiles:    []string{".goreleaser.yaml"},
			requiredAutomationPaths: []string{"Makefile"},
			reliability:             boolPtr(true),
			data:                    boolPtr(true),
			observability:           boolPtr(true),
			operations:              boolPtr(false),
			delivery:                boolPtr(true),
			change:                  boolPtr(true),
			maxChangedFiles:         20,
			maxChangedDirectories:   6,
			maxChangedLines:         600,
			minTestProdRatioPercent: 30,
		},
	}
}

func profileThresholdsFromConfig(cfg core.Config) profileThresholds {
	return profileThresholds{
		maxFileLines:            cfg.Checks.QualityRules.MaxFileLines,
		maxFunctionLines:        cfg.Checks.QualityRules.MaxFunctionLines,
		maxParameters:           cfg.Checks.QualityRules.MaxParameters,
		maxCyclomaticComplexity: cfg.Checks.QualityRules.MaxCyclomaticComplexity,
		cloneTokenThreshold:     cfg.Checks.QualityRules.CloneTokenThreshold,
		maxDeclsPerFile:         cfg.Checks.DesignRules.MaxDeclsPerFile,
		maxMethodsPerType:       cfg.Checks.DesignRules.MaxMethodsPerType,
		maxInterfaceMethods:     cfg.Checks.DesignRules.MaxInterfaceMethods,
		govulncheckMode:         cfg.Checks.SecurityRules.GovulncheckMode,
		requiredReleaseFiles:    cfg.Checks.CIRules.RequiredReleaseFiles,
		requiredAutomationPaths: cfg.Checks.CIRules.RequiredAutomationPaths,
		contracts:               cfg.Checks.Contracts,
		reliability:             cfg.Checks.Reliability,
		data:                    cfg.Checks.Data,
		observability:           cfg.Checks.Observability,
		operations:              cfg.Checks.Operations,
		delivery:                cfg.Checks.Delivery,
		change:                  cfg.Checks.Change,
		maxChangedFiles:         cfg.Checks.ChangeRules.MaxChangedFiles,
		maxChangedDirectories:   cfg.Checks.ChangeRules.MaxChangedDirectories,
		maxChangedLines:         cfg.Checks.ChangeRules.MaxChangedLines,
		minTestProdRatioPercent: cfg.Checks.ChangeRules.MinTestToProductionRatioPercent,
	}
}

func TestPolicyProfileDocumentationMatchesGeneratedComparison(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "checks.md"))
	if err != nil {
		t.Fatalf("read checks documentation: %v", err)
	}
	const start = "<!-- BEGIN GENERATED: policy-profile-comparison -->"
	const end = "<!-- END GENERATED: policy-profile-comparison -->"
	documentation := string(contents)
	startIndex := strings.Index(documentation, start)
	endIndex := strings.Index(documentation, end)
	if startIndex == -1 || endIndex == -1 || endIndex < startIndex {
		t.Fatal("checks documentation is missing the generated policy profile comparison")
	}
	got := documentation[startIndex : endIndex+len(end)]
	want := strings.TrimSpace(RenderPolicyProfileComparison())
	if got != want {
		t.Errorf("generated policy profile comparison is stale\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}
