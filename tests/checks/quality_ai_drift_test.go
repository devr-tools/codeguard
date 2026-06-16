package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityCheckWarnsForGoErrorStyleDrift(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wrap_one.go"), `package sample

import "fmt"

func One() error {
	if err := step(); err != nil {
		return fmt.Errorf("one: %w", err)
	}
	if err := step(); err != nil {
		return fmt.Errorf("again: %w", err)
	}
	return nil
}

func step() error { return nil }
`)
	writeFile(t, filepath.Join(dir, "wrap_two.go"), `package sample

import "fmt"

func Two() error {
	if err := step(); err != nil {
		return fmt.Errorf("two: %w", err)
	}
	return nil
}
`)
	writeFile(t, filepath.Join(dir, "drifted.go"), `package sample

import "errors"

func Three() error {
	if bad() {
		return errors.New("first failure")
	}
	return errors.New("second failure")
}

func bad() bool { return false }
`)

	report, err := codeguard.Run(context.Background(), qualityAITestConfig(dir, "quality-ai-go-errstyle"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.error-style-drift")
}

func TestQualityCheckAllowsGoWrapAdoptionInUnwrappedRepo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "plain_one.go"), `package sample

import "errors"

func One() error {
	if bad() {
		return errors.New("first failure")
	}
	return errors.New("second failure")
}

func bad() bool { return false }
`)
	writeFile(t, filepath.Join(dir, "plain_two.go"), `package sample

import "errors"

func Two() error {
	if bad() {
		return errors.New("third failure")
	}
	return errors.New("fourth failure")
}
`)
	writeFile(t, filepath.Join(dir, "adopter.go"), `package sample

import "fmt"

func Three() error {
	if err := step(); err != nil {
		return fmt.Errorf("three: %w", err)
	}
	if err := step(); err != nil {
		return fmt.Errorf("again: %w", err)
	}
	return nil
}

func step() error { return nil }
`)

	report, err := codeguard.Run(context.Background(), qualityAITestConfig(dir, "quality-ai-go-wrap-adoption"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.error-style-drift")
}

func TestQualityCheckWarnsForPythonBareExceptDrift(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "typed.py"), `def first():
    try:
        work()
    except ValueError:
        raise
    try:
        work()
    except (KeyError, TypeError):
        raise
    try:
        work()
    except RuntimeError:
        raise
`)
	writeFile(t, filepath.Join(dir, "drifted.py"), `def second():
    try:
        work()
    except:
        raise
`)

	cfg := qualityAITestConfig(dir, "quality-ai-py-errstyle")
	cfg.Targets[0].Language = "python"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.error-style-drift")
}

func TestQualityCheckWarnsForTypeScriptErrorClassDrift(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "custom.ts"), `export class ValidationError extends Error {}

export function checkOne(value: string): void {
  if (!value) {
    throw new ValidationError("one");
  }
  if (value.length > 10) {
    throw new ValidationError("two");
  }
  if (value.length > 20) {
    throw new ValidationError("three");
  }
}
`)
	writeFile(t, filepath.Join(dir, "drifted.ts"), `export function checkTwo(value: string): void {
  if (!value) {
    throw new Error("raw one");
  }
  if (value.length > 10) {
    throw new Error("raw two");
  }
}
`)

	cfg := qualityAITestConfig(dir, "quality-ai-ts-errstyle")
	cfg.Targets[0].Language = "typescript"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.error-style-drift")
}

func TestQualityCheckAllowsConsistentErrorStyles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wrap_one.go"), `package sample

import "fmt"

func One() error {
	if err := step(); err != nil {
		return fmt.Errorf("one: %w", err)
	}
	if err := step(); err != nil {
		return fmt.Errorf("two: %w", err)
	}
	if err := step(); err != nil {
		return fmt.Errorf("three: %w", err)
	}
	return nil
}

func step() error { return nil }
`)

	report, err := codeguard.Run(context.Background(), qualityAITestConfig(dir, "quality-ai-errstyle-clean"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.error-style-drift")
}

func TestQualityCheckWarnsForPythonNamingDrift(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "snake.py"), `def load_config():
    return 1

def parse_input():
    return 2

def write_output():
    return 3
`)
	writeFile(t, filepath.Join(dir, "drifted.py"), `def loadSettings():
    return 1

def parseValues():
    return 2
`)

	cfg := qualityAITestConfig(dir, "quality-ai-py-naming")
	cfg.Targets[0].Language = "python"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.naming-drift")
}

func TestQualityCheckAllowsConsistentNaming(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "snake.py"), `def load_config():
    return 1

def parse_input():
    return 2

def write_output():
    return 3
`)
	writeFile(t, filepath.Join(dir, "more_snake.py"), `def read_file():
    return 1

def close_file():
    return 2
`)

	cfg := qualityAITestConfig(dir, "quality-ai-naming-clean")
	cfg.Targets[0].Language = "python"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.naming-drift")
}

func TestQualityCheckHonorsDriftToggles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "typed.py"), `def first():
    try:
        work()
    except ValueError:
        raise
    try:
        work()
    except KeyError:
        raise
    try:
        work()
    except RuntimeError:
        raise

def load_config():
    return 1

def parse_input():
    return 2

def write_output():
    return 3
`)
	writeFile(t, filepath.Join(dir, "drifted.py"), `def secondThing():
    try:
        work()
    except:
        raise

def thirdThing():
    return 2
`)

	cfg := qualityAITestConfig(dir, "quality-ai-drift-toggles")
	cfg.Targets[0].Language = "python"
	disabled := false
	cfg.Checks.QualityRules.AIChecks.ErrorStyleDrift = &disabled
	cfg.Checks.QualityRules.AIChecks.NamingDrift = &disabled
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.error-style-drift")
	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.naming-drift")
}
