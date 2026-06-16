package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityCheckWarnsForAISwallowedErrorInGo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), `package sample

func run() error {
	err := doThing()
	_ = err
	return nil
}

func doThing() error { return nil }
`)

	report, err := codeguard.Run(context.Background(), qualityAITestConfig(dir, "quality-ai-go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.swallowed-error")
	assertSlopScoreArtifactPresent(t, report)
}

func TestQualityCheckWarnsForNarrativeCommentInGo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "comment.go"), `package sample

// Initialize the client.
func buildClient() {}
`)

	report, err := codeguard.Run(context.Background(), qualityAITestConfig(dir, "quality-ai-comment"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.narrative-comment")
}

func TestQualityCheckWarnsForEmptyCatchInTypeScript(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.ts"), `export function run() {
  try {
    work();
  } catch (err) {}
}

function work() {}
`)

	cfg := qualityAITestConfig(dir, "quality-ai-ts")
	cfg.Targets[0].Language = "typescript"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.swallowed-error")
}

func TestQualityCheckWarnsForPassOnlyExceptInPython(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.py"), `def run():
    try:
        do_work()
    except Exception:
        pass

def do_work():
    return None
`)

	cfg := qualityAITestConfig(dir, "quality-ai-py")
	cfg.Targets[0].Language = "python"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.swallowed-error")
}

func qualityAITestConfig(dir string, name string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	return cfg
}

func assertSlopScoreArtifactPresent(t *testing.T, report codeguard.Report) {
	t.Helper()
	for _, artifact := range report.Artifacts {
		if artifact.Kind != "slop_score" || artifact.SlopScore == nil {
			continue
		}
		if artifact.SlopScore.Score <= 0 || artifact.SlopScore.Signals <= 0 {
			t.Fatalf("unexpected slop score artifact: %#v", artifact.SlopScore)
		}
		return
	}
	t.Fatalf("expected slop_score artifact, got %#v", report.Artifacts)
}
