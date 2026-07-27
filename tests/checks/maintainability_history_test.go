package checks_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func initMaintainabilityHistoryRepo(t *testing.T) string {
	t.Helper()
	dir := initContractsRepo(t)
	writeFile(t, filepath.Join(dir, "risky.go"), maintainabilityHistorySource(0, "base"))
	writeFile(t, filepath.Join(dir, "partner_a.go"), "package sample\n\nfunc PartnerA() int { return 1 }\n")
	writeFile(t, filepath.Join(dir, "partner_b.go"), "package sample\n\nfunc PartnerB() int { return 1 }\n")
	writeFile(t, filepath.Join(dir, "partner_c.go"), "package sample\n\nfunc PartnerC() int { return 1 }\n")
	writeFile(t, filepath.Join(dir, "partner_d.go"), "package sample\n\nfunc PartnerD() int { return 1 }\n")
	commitAll(t, dir, "api base")

	commits := []struct {
		message  string
		partners []string
	}{
		{message: "fix api bug in risky flow", partners: []string{"partner_a.go", "partner_b.go", "partner_c.go", "partner_d.go"}},
		{message: "refactor risky flow", partners: []string{"partner_a.go", "partner_b.go", "partner_c.go"}},
		{message: "db cache update for risky flow", partners: []string{"partner_a.go", "partner_b.go", "partner_c.go"}},
		{message: "perf speed up risky flow", partners: []string{"partner_a.go", "partner_b.go", "partner_c.go"}},
		{message: "fix regression in risky flow", partners: []string{"partner_a.go", "partner_b.go", "partner_c.go"}},
	}
	for idx, commit := range commits {
		writeFile(t, filepath.Join(dir, "risky.go"), maintainabilityHistorySource(idx+1, commit.message))
		for _, partner := range commit.partners {
			writeFile(t, filepath.Join(dir, partner), fmt.Sprintf("package sample\n\nfunc %s() int { return %d }\n", partnerFunctionName(partner), idx+2))
		}
		commitAll(t, dir, commit.message)
	}

	runGit(t, dir, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(dir, "risky.go"), maintainabilityHistorySource(99, "feature change"))
	return dir
}

func maintainabilityHistorySource(version int, label string) string {
	lines := []string{
		"package sample",
		"",
		"func PublicRiskyAPI(input int) int {",
		"\tresult := input",
	}
	for idx := 0; idx < 12; idx++ {
		lines = append(lines,
			fmt.Sprintf("\tif result > %d {", idx),
			fmt.Sprintf("\t\tresult += %d", version+idx+1),
			"\t}",
		)
	}
	for idx := 0; idx < 30; idx++ {
		lines = append(lines, fmt.Sprintf("\tresult += %d // %s filler %02d", version+idx, label, idx))
	}
	lines = append(lines,
		"\treturn result",
		"}",
		"",
	)
	return strings.Join(lines, "\n")
}

func partnerFunctionName(path string) string {
	switch path {
	case "partner_a.go":
		return "PartnerA"
	case "partner_b.go":
		return "PartnerB"
	case "partner_c.go":
		return "PartnerC"
	default:
		return "PartnerD"
	}
}

func TestMaintainabilityHistoryHotspotRulesUseGitEvidence(t *testing.T) {
	dir := initMaintainabilityHistoryRepo(t)

	report := runMaintainabilityDeltaScan(t, qualityPrecisionConfig(dir))

	assertFindingRulePresent(t, report, "Code Quality", "maintainability.hotspot")
	assertFindingRulePresent(t, report, "Code Quality", "maintainability.high-churn-hotspot")
	assertFindingRulePresent(t, report, "Code Quality", "maintainability.repeat-defect-area")
	assertFindingRulePresent(t, report, "Code Quality", "maintainability.unstable-interface")
	assertFindingLevel(t, report, "Code Quality", "maintainability.high-churn-hotspot", "warn")

	finding := findFinding(t, report, "Code Quality", "maintainability.high-churn-hotspot")
	if finding.Metadata["commits"] == "" || finding.Metadata["churn"] == "" || finding.Metadata["decision_hints"] == "" {
		t.Fatalf("missing history evidence metadata: %#v", finding.Metadata)
	}
	if !strings.Contains(finding.Message, "churn") {
		t.Fatalf("finding message should include churn evidence: %q", finding.Message)
	}
}

func TestMaintainabilityHistoryUnavailableDoesNotFailScan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "plain.go"), "package sample\n\nfunc Value() int { return 1 }\n")

	report, err := codeguard.Run(context.Background(), qualityPrecisionConfig(dir))
	if err != nil {
		t.Fatalf("full scan without git history: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Code Quality", "maintainability.hotspot")
	assertSectionStatus(t, report, "Code Quality", "pass")
}
