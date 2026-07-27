package checks_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestRefactorDetectsBehaviorErrorAndSideEffectOrderChanges(t *testing.T) {
	dir := initChangeRepo(t)
	before := `package service

import (
	"errors"
	"fmt"
)

type Repo interface { Save() error }
type Bus interface { Publish(string) }
type User struct{}

func Authorize(User) bool { return true }

func Process(repo Repo, bus Bus, user User) error {
	if !Authorize(user) {
		return errors.New("denied")
	}
	if err := repo.Save(); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	bus.Publish("saved")
	return nil
}
`
	after := `package app

type Repo interface { Save() error }
type Bus interface { Publish(string) }
type User struct{}

func Authorize(User) bool { return true }

func Process(repo Repo, bus Bus, user User) error {
	if !Authorize(user) {
		return nil
	}
	bus.Publish("saved")
	if err := repo.Save(); err != nil {
		return err
	}
	return nil
}
`
	writeFile(t, filepath.Join(dir, "service", "processor.go"), before)
	commitAll(t, dir, "base")

	if err := os.Remove(filepath.Join(dir, "service", "processor.go")); err != nil {
		t.Fatalf("remove old processor: %v", err)
	}
	writeFile(t, filepath.Join(dir, "app", "processor.go"), after)
	runGit(t, dir, "add", "-N", "app/processor.go")

	cfg := refactorTestConfig(t, dir, "go")
	report := runChangeDiff(t, cfg)

	assertFindingRulePresent(t, report, "Change Safety", "refactor.behavior-change-detected")
	assertFindingRulePresent(t, report, "Change Safety", "refactor.error-path-changed")
	assertFindingRulePresent(t, report, "Change Safety", "refactor.side-effect-order-changed")
}

func TestRefactorDetectsBehaviorErrorAndSideEffectOrderAcrossNonGoLanguages(t *testing.T) {
	for _, tc := range refactorBehaviorCases() {
		t.Run(tc.name, func(t *testing.T) {
			dir := initChangeRepo(t)
			writeFile(t, filepath.Join(dir, tc.path), tc.before)
			commitAll(t, dir, "base")
			writeFile(t, filepath.Join(dir, tc.path), tc.after)

			report := runChangeDiff(t, refactorTestConfig(t, dir, tc.language))

			assertFindingRulePresent(t, report, "Change Safety", "refactor.behavior-change-detected")
			assertFindingRulePresent(t, report, "Change Safety", "refactor.error-path-changed")
			assertFindingRulePresent(t, report, "Change Safety", "refactor.side-effect-order-changed")
		})
	}
}

func TestRefactorDetectsPublicContractAndVisibilityExpansion(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "pkg", "client", "api.go"), `package client

func Price(value int) int { return value }

func normalize(value int) int { return value }
`)
	commitAll(t, dir, "base")
	writeFile(t, filepath.Join(dir, "pkg", "client", "api.go"), `package client

func Price(value int, currency string) int { return value }

func Normalize(value int) int { return value }
`)

	report := runChangeDiff(t, refactorTestConfig(t, dir, "go"))

	assertFindingRulePresent(t, report, "Change Safety", "refactor.public-contract-changed")
	assertFindingRulePresent(t, report, "Change Safety", "refactor.visibility-expanded")
}

func TestRefactorDetectsTestCoverageReduced(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "pricing", "price_test.go"), `package pricing

import "testing"

func TestPriceBase(t *testing.T) {
	if got := 10; got != 10 { t.Fatal(got) }
}

func TestPriceDiscount(t *testing.T) {
	if got := 9; got != 9 { t.Fatal(got) }
}
`)
	commitAll(t, dir, "base")
	writeFile(t, filepath.Join(dir, "pricing", "price_test.go"), `package pricing

import "testing"

func TestPriceBase(t *testing.T) {
	if got := 10; got != 10 { t.Fatal(got) }
}
`)

	report := runChangeDiff(t, refactorTestConfig(t, dir, "go"))

	assertFindingRulePresent(t, report, "Change Safety", "refactor.test-coverage-reduced")
}

func TestRefactorDetectsDependencyDirectionWorsenedAcrossLanguages(t *testing.T) {
	cases := []struct {
		name     string
		language string
		path     string
		before   string
		after    string
	}{
		{
			name:     "go",
			language: "go",
			path:     "internal/domain/order.go",
			before:   "package domain\n\nfunc Total(value int) int {\n\treturn value\n}\n",
			after:    "package domain\n\nimport \"net/http\"\n\nfunc Total(value int) int {\n\t_ = http.DefaultClient\n\treturn value\n}\n",
		},
		{
			name:     "python",
			language: "python",
			path:     "app/domain/order.py",
			before:   "def total(value):\n    return value\n",
			after:    "import requests\n\n\ndef total(value):\n    return requests.get('https://example.test').status_code + value\n",
		},
		{
			name:     "typescript",
			language: "typescript",
			path:     "src/domain/order.ts",
			before:   "export function total(value: number) {\n  return value;\n}\n",
			after:    "import axios from 'axios';\n\nexport function total(value: number) {\n  return value;\n}\n",
		},
		{
			name:     "javascript",
			language: "javascript",
			path:     "src/domain/order.js",
			before:   "export function total(value) {\n  return value;\n}\n",
			after:    "import axios from 'axios';\n\nexport function total(value) {\n  return value;\n}\n",
		},
		{
			name:     "cpp",
			language: "c++",
			path:     "src/domain/order.cpp",
			before:   "int total(int value) {\n  return value;\n}\n",
			after:    "#include <filesystem>\n\nint total(int value) {\n  return value;\n}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := initChangeRepo(t)
			writeFile(t, filepath.Join(dir, tc.path), tc.before)
			commitAll(t, dir, "base")
			writeFile(t, filepath.Join(dir, tc.path), tc.after)

			report := runChangeDiff(t, refactorTestConfig(t, dir, tc.language))

			assertFindingRulePresent(t, report, "Change Safety", "refactor.dependency-direction-worsened")
		})
	}
}

func TestRefactorDetectsDuplicateImplementationAndDeadPathLeftBehind(t *testing.T) {
	dir := initChangeRepo(t)
	body := `package service

func OldTotal(value int) int {
	total := value
	if total > 100 {
		total = total - 10
	}
	if total < 0 {
		total = 0
	}
	return total
}
`
	writeFile(t, filepath.Join(dir, "service", "old_total.go"), body)
	writeFile(t, filepath.Join(dir, "service", "legacy.go"), "package service\n\nfunc UseLegacy() int { return 1 }\n")
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "service", "new_total.go"), `package service

func NewTotal(value int) int {
	total := value
	if total > 100 {
		total = total - 10
	}
	if total < 0 {
		total = 0
	}
	return total
}
`)
	writeFile(t, filepath.Join(dir, "service", "legacy.go"), `package service

func UseLegacy() int {
	if false {
		return legacyCompatibility()
	}
	return 1
}

func legacyCompatibility() int { return 0 }
`)
	runGit(t, dir, "add", "-N", "service/new_total.go")

	report := runChangeDiff(t, refactorTestConfig(t, dir, "go"))

	assertFindingRulePresent(t, report, "Change Safety", "refactor.duplicate-implementation-left-behind")
	assertFindingRulePresent(t, report, "Change Safety", "refactor.dead-path-left-behind")
}

func TestRefactorBehaviorPreservingMoveWithTestsDoesNotEmitRefactorFindings(t *testing.T) {
	dir := initChangeRepo(t)
	before := `package worker

type Repo interface { Save() error }
type Bus interface { Publish(string) }

func Process(repo Repo, bus Bus) error {
	if err := repo.Save(); err != nil {
		return err
	}
	bus.Publish("saved")
	return nil
}
`
	writeFile(t, filepath.Join(dir, "service", "worker.go"), before)
	commitAll(t, dir, "base")

	if err := os.Remove(filepath.Join(dir, "service", "worker.go")); err != nil {
		t.Fatalf("remove old worker: %v", err)
	}
	writeFile(t, filepath.Join(dir, "app", "worker.go"), before)
	writeFile(t, filepath.Join(dir, "app", "worker_test.go"), "package worker\n\nimport \"testing\"\n\nfunc TestProcess(t *testing.T) {}\n")
	runGit(t, dir, "add", "-N", "app/worker.go", "app/worker_test.go")

	report := runChangeDiff(t, refactorTestConfig(t, dir, "go"))

	for _, ruleID := range []string{
		"refactor.behavior-change-detected",
		"refactor.public-contract-changed",
		"refactor.test-coverage-reduced",
		"refactor.error-path-changed",
		"refactor.side-effect-order-changed",
		"refactor.visibility-expanded",
		"refactor.dependency-direction-worsened",
		"refactor.duplicate-implementation-left-behind",
		"refactor.dead-path-left-behind",
	} {
		assertFindingRuleAbsent(t, report, "Change Safety", ruleID)
	}
}

func refactorTestConfig(t *testing.T, dir string, language string) codeguard.Config {
	t.Helper()
	cfg := changeSafetyTestConfig("refactor-test", dir)
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest = boolValue(false)
	cfg.Checks.ChangeRules.DetectFailurePathMissing = boolValue(false)
	cfg.Checks.ChangeRules.DetectHardwiredDependency = boolValue(false)
	cfg.Checks.ChangeRules.DetectNondeterministicDomain = boolValue(false)
	return cfg
}

type refactorBehaviorCase struct {
	name     string
	language string
	path     string
	before   string
	after    string
}

func refactorBehaviorCases() []refactorBehaviorCase {
	return []refactorBehaviorCase{
		{
			name:     "python",
			language: "python",
			path:     "app/refactor_processor.py",
			before:   "def process(repo, bus, allowed):\n    if not allowed:\n        raise Exception('denied')\n    repo.save()\n    bus.publish('saved')\n    return True\n",
			after:    "def process(repo, bus, allowed):\n    if not allowed:\n        return True\n    bus.publish('saved')\n    repo.save()\n    return True\n",
		},
		{
			name:     "typescript",
			language: "typescript",
			path:     "src/refactorProcessor.ts",
			before:   "export function process(repo, bus, allowed) {\n  if (!allowed) { throw new Error('denied') }\n  repo.save()\n  bus.publish('saved')\n  return true\n}\n",
			after:    "export function process(repo, bus, allowed) {\n  if (!allowed) { return true }\n  bus.publish('saved')\n  repo.save()\n  return true\n}\n",
		},
		{
			name:     "javascript",
			language: "javascript",
			path:     "src/refactorProcessor.js",
			before:   "export function process(repo, bus, allowed) {\n  if (!allowed) { throw new Error('denied') }\n  repo.save()\n  bus.publish('saved')\n  return true\n}\n",
			after:    "export function process(repo, bus, allowed) {\n  if (!allowed) { return true }\n  bus.publish('saved')\n  repo.save()\n  return true\n}\n",
		},
		{
			name:     "cpp",
			language: "c++",
			path:     "src/refactor_processor.cpp",
			before:   "bool Process(Repo& repo, Bus& bus, bool allowed) {\n  if (!allowed) { throw std::runtime_error(\"denied\"); }\n  repo.Save();\n  bus.Publish(\"saved\");\n  return true;\n}\n",
			after:    "bool Process(Repo& repo, Bus& bus, bool allowed) {\n  if (!allowed) { return true; }\n  bus.Publish(\"saved\");\n  repo.Save();\n  return true;\n}\n",
		},
	}
}
