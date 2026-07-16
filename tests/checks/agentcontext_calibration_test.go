package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// The agent_docs component is gated on substance: an empty agent doc no
// longer banks the full 25 points just by existing.
func TestRepoLegibilityAgentDocsComponentRequiresSubstance(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "\n\n\n")

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "agent-docs-empty"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	artifact := requireRepoLegibilityArtifact(t, report)
	component := legibilityComponent(t, artifact, "agent_docs")
	if component.Score != 0 {
		t.Fatalf("empty CLAUDE.md agent_docs score = %d, want 0: %+v", component.Score, component)
	}
	if !strings.Contains(component.Detail, "0 non-blank lines") || !strings.Contains(component.Detail, "substance 0/25") {
		t.Fatalf("agent_docs detail should explain the substance formula: %q", component.Detail)
	}
}

// Substance credit scales linearly up to 10 non-blank lines.
func TestRepoLegibilityAgentDocsComponentScalesWithContent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), strings.Repeat("guidance line without references\n", 4))

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "agent-docs-partial"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	artifact := requireRepoLegibilityArtifact(t, report)
	// 4 non-blank lines of 10 required: substance 25*4/10 = 10.
	if component := legibilityComponent(t, artifact, "agent_docs"); component.Score != 10 {
		t.Fatalf("agent_docs score = %d, want 10: %+v", component.Score, component)
	}
}

// A substantial agent doc riddled with stale references loses agent_docs
// credit on top of the shared doc_accuracy penalty.
func TestRepoLegibilityAgentDocsComponentPenalizesDrift(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	lines := make([]string, 0, 12)
	for i := 0; i < 10; i++ {
		lines = append(lines, "guidance line without references")
	}
	lines = append(lines, "Edit `internal/gone/one.go` and `internal/gone/two.go`.")
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), strings.Join(lines, "\n")+"\n")

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "agent-docs-drift-penalty"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	artifact := requireRepoLegibilityArtifact(t, report)
	// Full substance (11 non-blank lines), minus 2 per unresolvable agent-doc
	// reference: 25 - 2*2 = 21.
	component := legibilityComponent(t, artifact, "agent_docs")
	if component.Score != 21 || !strings.Contains(component.Detail, "2 unresolvable references") {
		t.Fatalf("agent_docs score = %d, want 21: %+v", component.Score, component)
	}
}

// doc_accuracy scales with the broken share instead of the old flat -4 that
// saturated at 5 references: 2 broken of 10 costs 4 points, not 8.
func TestRepoLegibilityDocAccuracyScalesProportionally(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	for i := 0; i < 8; i++ {
		writeFile(t, filepath.Join(dir, "scripts", "tool"+string(rune('a'+i))+".sh"), "#!/bin/sh\necho ok\n")
	}
	commands := []string{"# demo", "", "```bash"}
	for i := 0; i < 8; i++ {
		commands = append(commands, "./scripts/tool"+string(rune('a'+i))+".sh")
	}
	commands = append(commands, "./scripts/gone-one.sh", "./scripts/gone-two.sh", "```")
	writeFile(t, filepath.Join(dir, "README.md"), strings.Join(commands, "\n")+"\n")

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "doc-accuracy-proportional"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	artifact := requireRepoLegibilityArtifact(t, report)
	// 2 broken of 10 references: penalty round(20*2/10) = 4.
	component := legibilityComponent(t, artifact, "doc_accuracy")
	if component.Score != 16 || !strings.Contains(component.Detail, "2 of 10 doc references unresolvable") {
		t.Fatalf("doc_accuracy score = %d, want 16: %+v", component.Score, component)
	}
}

// context_economy degrades gradually: 10% oversized used to zero the whole
// component; under the linear ramp to 25% it now costs 10 of 25 points.
func TestRepoLegibilityContextEconomySoftensCurve(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# CLAUDE.md\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n")
	writeFile(t, filepath.Join(dir, "big.go"), goFileWithLines(40))
	for i := 0; i < 9; i++ {
		writeFile(t, filepath.Join(dir, "pkg"+string(rune('a'+i)), "small"+string(rune('a'+i))+".go"), "package fixture\n")
	}

	cfg := agentContextTestConfig(dir, "economy-soft-curve")
	cfg.Checks.ContextRules.MaxFileLines = 30

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	artifact := requireRepoLegibilityArtifact(t, report)
	// 1 of 10 source files oversized: penalty round(100*1/10) = 10.
	component := legibilityComponent(t, artifact, "context_economy")
	if component.Score != 15 || !strings.Contains(component.Detail, "1 of 10") {
		t.Fatalf("context_economy score = %d, want 15: %+v", component.Score, component)
	}
}

// Conventional basenames (index.ts, __init__.py, ...) are navigation noise an
// agent expects; they must not fire findings or drain the navigability score.
func TestRepoLegibilityNavigabilityIgnoresConventionalBasenames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# CLAUDE.md\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n")
	for _, sub := range []string{"api", "web", "cli", "db", "auth"} {
		writeFile(t, filepath.Join(dir, sub, "index.ts"), "export const ns = \""+sub+"\";\n")
	}

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "conventional-basenames"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Agent Context", "context.ambiguous-symbol")
	artifact := requireRepoLegibilityArtifact(t, report)
	if component := legibilityComponent(t, artifact, "navigability"); component.Score != 20 {
		t.Fatalf("navigability score = %d, want 20 (index.ts is conventional): %+v", component.Score, component)
	}
}

// context_rules.ambiguous_symbol_ignore replaces the default ignore list:
// custom entries take effect and the built-in defaults stop applying.
func TestRepoLegibilityAmbiguousIgnoreListIsConfigurable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# CLAUDE.md\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n")
	for _, sub := range []string{"api", "web", "cli", "db"} {
		writeFile(t, filepath.Join(dir, sub, "utils.ts"), "export const ns = \""+sub+"\";\n")
		writeFile(t, filepath.Join(dir, sub, "index.ts"), "export default \""+sub+"\";\n")
	}

	cfg := agentContextTestConfig(dir, "ambiguous-ignore-config")
	cfg.Checks.ContextRules.AmbiguousSymbolIgnore = []string{"utils.ts"}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// utils.ts is now ignored; index.ts is flagged because the custom list
	// replaced the defaults.
	messages := agentContextRuleMessages(report, "context.ambiguous-symbol")
	if len(messages) != 1 || !strings.Contains(messages[0], `"index.ts"`) {
		t.Fatalf("expected exactly one index.ts finding under the replaced ignore list, got: %v", messages)
	}
}
