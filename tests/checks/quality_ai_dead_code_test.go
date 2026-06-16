package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityCheckWarnsForCodeAfterReturnInGo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "unreachable.go"), `package sample

func Run() int {
	return 1
	cleanup()
}

func cleanup() {}
`)

	report, err := codeguard.Run(context.Background(), qualityAITestConfig(dir, "quality-ai-go-unreachable"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.dead-code")
}

func TestQualityCheckWarnsForUnusedPrivateGoFunction(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), `package sample

func Run() int {
	return used()
}

func used() int { return 1 }

func orphanHelper() int { return 2 }
`)

	report, err := codeguard.Run(context.Background(), qualityAITestConfig(dir, "quality-ai-go-unused"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.dead-code")
}

func TestQualityCheckAllowsReachableAndReferencedGoCode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), `package sample

func Run(flag bool) int {
	if flag {
		return 1
	}
	return helper()
}

func helper() int { return 2 }
`)

	report, err := codeguard.Run(context.Background(), qualityAITestConfig(dir, "quality-ai-go-clean"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.dead-code")
}

func TestQualityCheckWarnsForCodeAfterReturnInPython(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.py"), `def run():
    return 1
    print("never happens")
`)

	cfg := qualityAITestConfig(dir, "quality-ai-py-unreachable")
	cfg.Targets[0].Language = "python"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.dead-code")
}

func TestQualityCheckWarnsForUnusedPrivatePythonFunction(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.py"), `def run():
    return 1

def _orphan_helper():
    return 2
`)

	cfg := qualityAITestConfig(dir, "quality-ai-py-unused")
	cfg.Targets[0].Language = "python"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.dead-code")
}

func TestQualityCheckAllowsBranchedReturnsInPython(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.py"), `def run(flag):
    if flag:
        return 1
    else:
        return 2

def use_helper():
    return _helper()

def _helper():
    return 3
`)

	cfg := qualityAITestConfig(dir, "quality-ai-py-clean")
	cfg.Targets[0].Language = "python"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.dead-code")
}

func TestQualityCheckWarnsForCodeAfterReturnInTypeScript(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.ts"), `export function run(): number {
  return 1;
  cleanup();
}

export function cleanup(): void {}
`)

	cfg := qualityAITestConfig(dir, "quality-ai-ts-unreachable")
	cfg.Targets[0].Language = "typescript"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.dead-code")
}

func TestQualityCheckWarnsForUnusedLocalTypeScriptFunction(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.ts"), `export function run(): number {
  return 1;
}

function orphanHelper(): number {
  return 2;
}
`)

	cfg := qualityAITestConfig(dir, "quality-ai-ts-unused")
	cfg.Targets[0].Language = "typescript"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.dead-code")
}

func TestQualityCheckAllowsReachableTypeScriptCode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.ts"), `export function run(flag: boolean): number {
  if (flag) {
    return 1;
  }
  return helper();
}

function helper(): number {
  return 2;
}
`)

	cfg := qualityAITestConfig(dir, "quality-ai-ts-clean")
	cfg.Targets[0].Language = "typescript"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.dead-code")
}

func TestQualityCheckHonorsDeadCodeToggle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "unreachable.go"), `package sample

func Run() int {
	return 1
	cleanup()
}

func cleanup() {}
`)

	cfg := qualityAITestConfig(dir, "quality-ai-dead-toggle")
	disabled := false
	cfg.Checks.QualityRules.AIChecks.DeadCode = &disabled
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.dead-code")
}
