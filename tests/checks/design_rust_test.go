package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	designpkg "github.com/devr-tools/codeguard/internal/codeguard/checks/design"
	supportpkg "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestRustDesignFindingsWarnForGenericModuleName(t *testing.T) {
	findings := designpkg.RustFindingsForFile(rustDesignTestEnv(4, 4), "src/utils.rs", []byte("pub fn answer() -> i32 { 42 }\n"))

	assertRustFindingRulePresent(t, findings, "design.rust.generic-module-name")
}

func TestRustDesignFindingsWarnForTooManyImplMethodsAcrossBlocks(t *testing.T) {
	source := "" +
		"pub struct Service;\n" +
		"\n" +
		"impl Service {\n" +
		"    fn one(&self) {}\n" +
		"    fn two(&self) {}\n" +
		"}\n" +
		"\n" +
		"impl Service {\n" +
		"    fn three(&self) {}\n" +
		"}\n"

	findings := designpkg.RustFindingsForFile(rustDesignTestEnv(2, 4), "src/service.rs", []byte(source))

	assertRustFindingRulePresent(t, findings, "design.rust.max-methods-per-type")
	assertRustFindingMessageContains(t, findings, "Service", "3 impl methods")
}

func TestRustDesignFindingsWarnForLargeTraitSurface(t *testing.T) {
	source := "" +
		"pub trait Client {\n" +
		"    type Item;\n" +
		"    const LIMIT: usize;\n" +
		"    fn run(&self);\n" +
		"}\n"

	findings := designpkg.RustFindingsForFile(rustDesignTestEnv(4, 2), "src/client.rs", []byte(source))

	assertRustFindingRulePresent(t, findings, "design.rust.max-trait-members")
	assertRustFindingMessageContains(t, findings, "Client", "3 members")
}

func TestRustDesignFindingsPassForCrateRootAndSmallSurfaces(t *testing.T) {
	source := "" +
		"pub trait Client {\n" +
		"    type Item;\n" +
		"    fn run(&self);\n" +
		"}\n" +
		"\n" +
		"pub struct Service;\n" +
		"\n" +
		"impl Service {\n" +
		"    fn one(&self) {}\n" +
		"    fn two(&self) {}\n" +
		"}\n"

	findings := designpkg.RustFindingsForFile(rustDesignTestEnv(2, 2), "src/lib.rs", []byte(source))

	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

func TestDesignCheckWarnsForRustTypeSurfaceViaCodeguardRun(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "ports.rs"), "pub trait Client {\n    type Item;\n    const LIMIT: usize;\n    fn run(&self);\n}\n")
	writeFile(t, filepath.Join(dir, "src", "service.rs"), "pub struct Service;\n\nimpl Service {\n    fn one(&self) {}\n    fn two(&self) {}\n    fn three(&self) {}\n}\n")

	cfg := graphTestConfig("design-rust-native-heuristics", dir, "rust")
	cfg.Checks.DesignRules.MaxMethodsPerType = 2
	cfg.Checks.DesignRules.MaxInterfaceMethods = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "warn")
	assertFindingRulePresent(t, report, "Design Patterns", "design.rust.max-trait-members")
	assertFindingRulePresent(t, report, "Design Patterns", "design.rust.max-methods-per-type")
}

func rustDesignTestEnv(maxMethods int, maxTraitMembers int) supportpkg.Context {
	return supportpkg.Context{
		Config: core.Config{
			Checks: core.CheckConfig{
				DesignRules: core.DesignRulesConfig{
					MaxMethodsPerType:     maxMethods,
					MaxInterfaceMethods:   maxTraitMembers,
					ForbiddenPackageNames: []string{"util", "utils", "common"},
				},
			},
		},
		NewFinding: func(input supportpkg.FindingInput) core.Finding {
			return core.Finding{
				RuleID:     input.RuleID,
				Level:      input.Level,
				Message:    input.Message,
				Path:       input.Path,
				Line:       input.Line,
				Column:     input.Column,
				Confidence: input.Confidence,
			}
		},
	}
}

func assertRustFindingRulePresent(t *testing.T, findings []core.Finding, ruleID string) {
	t.Helper()
	for _, finding := range findings {
		if finding.RuleID == ruleID {
			return
		}
	}
	t.Fatalf("expected rule %q in findings %+v", ruleID, findings)
}

func assertRustFindingMessageContains(t *testing.T, findings []core.Finding, needles ...string) {
	t.Helper()
	for _, finding := range findings {
		matched := true
		for _, needle := range needles {
			if !strings.Contains(finding.Message, needle) {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("expected finding message containing %q, got %+v", needles, findings)
}
