package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestRepoLegibilityArtifactScoresFullyLegibleRepo(t *testing.T) {
	dir := t.TempDir()
	writeLegibleRepoFixture(t, dir)

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "legibility-full"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	artifact := requireRepoLegibilityArtifact(t, report)
	if artifact.RepoLegibility.Score != 100 {
		t.Fatalf("score = %d, want 100: %+v", artifact.RepoLegibility.Score, artifact.RepoLegibility.Components)
	}
	if got := len(artifact.RepoLegibility.Components); got != 5 {
		t.Fatalf("components = %d, want 5", got)
	}
	if component := legibilityComponent(t, artifact, "agent_docs"); component.Score != 25 || !strings.Contains(component.Detail, "CLAUDE.md") {
		t.Fatalf("unexpected agent_docs component: %+v", component)
	}
}

func TestRepoLegibilityArtifactExplainsPenalties(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n\n```bash\n./scripts/gone.sh\n```\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "legibility-penalties"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	artifact := requireRepoLegibilityArtifact(t, report)
	// agent_docs 0/25, readme 10/10, doc_accuracy 16/20 (one broken
	// reference), context_economy 25/25, navigability 20/20.
	if artifact.RepoLegibility.Score != 71 {
		t.Fatalf("score = %d, want 71: %+v", artifact.RepoLegibility.Score, artifact.RepoLegibility.Components)
	}
	if component := legibilityComponent(t, artifact, "agent_docs"); component.Score != 0 {
		t.Fatalf("agent_docs score = %d, want 0", component.Score)
	}
	if component := legibilityComponent(t, artifact, "doc_accuracy"); component.Score != 16 || !strings.Contains(component.Detail, "1 unresolvable") {
		t.Fatalf("unexpected doc_accuracy component: %+v", component)
	}
	if component := legibilityComponent(t, artifact, "readme"); component.Score != 10 {
		t.Fatalf("readme score = %d, want 10", component.Score)
	}
}

func TestRepoLegibilityArtifactEmittedEvenWhenRulesAreMuted(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")

	off := false
	cfg := agentContextTestConfig(dir, "legibility-muted")
	cfg.Checks.ContextRules.DetectMissingAgentDocs = &off
	cfg.Checks.ContextRules.DetectAgentDocsDrift = &off
	cfg.Checks.ContextRules.DetectReadmeDrift = &off
	cfg.Checks.ContextRules.DetectOversizedFiles = &off
	cfg.Checks.ContextRules.DetectAmbiguousSymbols = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "pass")
	artifact := requireRepoLegibilityArtifact(t, report)
	// agent_docs 0/25 and readme 0/10 still count against the score even
	// though the findings are muted: the artifact reports reality.
	if artifact.RepoLegibility.Score != 65 {
		t.Fatalf("score = %d, want 65: %+v", artifact.RepoLegibility.Score, artifact.RepoLegibility.Components)
	}
}

func TestRepoLegibilityArtifactCountsOversizedAndAmbiguousRatios(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# CLAUDE.md\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n")
	writeFile(t, filepath.Join(dir, "big.go"), goFileWithLines(40))
	for _, sub := range []string{"api", "web", "cli", "db"} {
		writeFile(t, filepath.Join(dir, sub, "utils.ts"), "export const ns = \""+sub+"\";\n")
	}

	cfg := agentContextTestConfig(dir, "legibility-ratios")
	cfg.Checks.ContextRules.MaxFileLines = 30

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	artifact := requireRepoLegibilityArtifact(t, report)
	// 1 of 5 source files oversized: penalty min(25, 25*1*10/5) = 25.
	if component := legibilityComponent(t, artifact, "context_economy"); component.Score != 0 || !strings.Contains(component.Detail, "1 of 5") {
		t.Fatalf("unexpected context_economy component: %+v", component)
	}
	// 4 of 5 source files share a basename: penalty min(20, 20*4*5/5) = 20.
	if component := legibilityComponent(t, artifact, "navigability"); component.Score != 0 || !strings.Contains(component.Detail, "4 files share 1 ambiguous basenames") {
		t.Fatalf("unexpected navigability component: %+v", component)
	}
}
