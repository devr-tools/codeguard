package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// runGoSecurityScan writes one Go source file and runs a security-only scan
// over it.
func runGoSecurityScan(t *testing.T, name string, sourceLines []string) codeguard.Report {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), strings.Join(sourceLines, "\n")+"\n")

	report, err := codeguard.Run(context.Background(), securityOnlyConfig(name, dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func TestSecurityGoDetectionPrecision(t *testing.T) {
	if len(securityGoDetectionCases()) == 0 {
		t.Fatal("expected detection cases")
	}
	runSecurityGoCases(t, "security-go-detection", securityGoDetectionCases())
}

func TestSecurityGoFallbackScansMaskedSourceWhenParseFails(t *testing.T) {
	if len(securityGoFallbackCases()) == 0 {
		t.Fatal("expected fallback cases")
	}
	runSecurityGoCases(t, "security-go-fallback", securityGoFallbackCases())
}

func runSecurityGoCases(t *testing.T, name string, cases []securityGoCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := runGoSecurityScan(t, name, tc.source)
			assertSectionStatus(t, report, "Security", tc.status)
			for _, ruleID := range tc.present {
				assertFindingRulePresent(t, report, "Security", ruleID)
			}
			for _, ruleID := range tc.absent {
				assertFindingRuleAbsent(t, report, "Security", ruleID)
			}
		})
	}
}
