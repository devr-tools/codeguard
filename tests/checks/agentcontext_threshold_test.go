package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// writeLowLegibilityFixture builds a repo that scores well below 100: no
// agent docs and a README whose only command reference is broken (agent_docs
// 0/25, readme 10/10, doc_accuracy 0/20, context_economy 25/25, navigability
// 20/20 = 55).
func writeLowLegibilityFixture(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n\n```bash\n./scripts/gone.sh\n```\n")
}

func TestLegibilityThresholdWarnsWhenScoreFallsBelow(t *testing.T) {
	dir := t.TempDir()
	writeLowLegibilityFixture(t, dir)

	cfg := agentContextTestConfig(dir, "legibility-threshold-warn")
	cfg.Checks.ContextRules.LegibilityWarnThreshold = 80

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "warn")
	messages := agentContextRuleMessages(report, "context.legibility-threshold")
	if len(messages) != 1 {
		t.Fatalf("legibility-threshold findings = %d, want 1: %v", len(messages), messages)
	}
	for _, needle := range []string{"score 55", "warn threshold 80", "agent_docs 0/25", "doc_accuracy 0/20", "navigability 20/20"} {
		if !strings.Contains(messages[0], needle) {
			t.Fatalf("threshold message missing %q: %q", needle, messages[0])
		}
	}
}

func TestLegibilityThresholdFailsBelowFailThreshold(t *testing.T) {
	dir := t.TempDir()
	writeLowLegibilityFixture(t, dir)

	cfg := agentContextTestConfig(dir, "legibility-threshold-fail")
	cfg.Checks.ContextRules.LegibilityWarnThreshold = 80
	cfg.Checks.ContextRules.LegibilityFailThreshold = 60

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "fail")
	messages := agentContextRuleMessages(report, "context.legibility-threshold")
	if len(messages) != 1 || !strings.Contains(messages[0], "fail threshold 60") {
		t.Fatalf("expected one fail-level threshold finding, got: %v", messages)
	}
}

func TestLegibilityThresholdDisabledAtZero(t *testing.T) {
	dir := t.TempDir()
	writeLowLegibilityFixture(t, dir)

	// Thresholds default to 0: the score is published but never enforced.
	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "legibility-threshold-off"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Agent Context", "context.legibility-threshold")
}

func TestLegibilityThresholdQuietWhenScoreMeetsBar(t *testing.T) {
	dir := t.TempDir()
	writeLegibleRepoFixture(t, dir)

	cfg := agentContextTestConfig(dir, "legibility-threshold-pass")
	cfg.Checks.ContextRules.LegibilityWarnThreshold = 80

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "pass")
	assertFindingRuleAbsent(t, report, "Agent Context", "context.legibility-threshold")
}

func TestLegibilityThresholdConfigValidation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")

	cfg := agentContextTestConfig(dir, "legibility-threshold-invalid")
	// Fail must sit at or below warn: legibility is good-high, so the fail
	// bar is the lower one.
	cfg.Checks.ContextRules.LegibilityWarnThreshold = 50
	cfg.Checks.ContextRules.LegibilityFailThreshold = 70

	if _, err := codeguard.Run(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "legibility_fail_threshold") {
		t.Fatalf("expected legibility_fail_threshold validation error, got: %v", err)
	}

	cfg.Checks.ContextRules.LegibilityFailThreshold = 0
	cfg.Checks.ContextRules.LegibilityWarnThreshold = 101
	if _, err := codeguard.Run(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "legibility_warn_threshold") {
		t.Fatalf("expected legibility_warn_threshold validation error, got: %v", err)
	}
}
