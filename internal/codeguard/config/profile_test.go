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

func expectedProfileThresholds() map[string]profileThresholds {
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
		},
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
