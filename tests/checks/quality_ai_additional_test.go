package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityCheckWarnsForHallucinatedGoImport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/sample\n\ngo 1.23.0\n")
	writeFile(t, filepath.Join(dir, "service.go"), `package sample

import "github.com/imaginary/module/client"

func run() {}
`)

	report, err := codeguard.Run(context.Background(), qualityAITestConfig(dir, "quality-ai-go-import"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.hallucinated-import")
	assertFindingConfidence(t, report, "Code Quality", "quality.ai.hallucinated-import", "high")
}

func TestQualityCheckWarnsForHallucinatedTypeScriptImport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"fixture","dependencies":{"react":"18.0.0"}}`)
	writeFile(t, filepath.Join(dir, "src", "app.ts"), `import missing from "totally-missing-package";

export const value = missing;
`)

	cfg := qualityAITestConfig(dir, "quality-ai-ts-import")
	cfg.Targets[0].Language = "typescript"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.hallucinated-import")
}

func TestQualityCheckWarnsForDeadCode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "dead.go"), `package sample

func run() {
	if false {
		doThing()
	}
}

func doThing() {}
`)

	report, err := codeguard.Run(context.Background(), qualityAITestConfig(dir, "quality-ai-dead"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.dead-code")
}

func TestQualityCheckWarnsForOverMockedGoTest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/sample\n\ngo 1.23.0\n")
	writeFile(t, filepath.Join(dir, "service_test.go"), `package sample

import "testing"

func TestRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := NewMockClient(ctrl)
	client.EXPECT().Call().Return(nil)
	client.EXPECT().Close().Return(nil)
	mockValue := mock.Anything
	_ = mockValue
	_ = client
}
`)

	report, err := codeguard.Run(context.Background(), qualityAITestConfig(dir, "quality-ai-overmock-go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.over-mocked-test")
}

func TestQualityCheckWarnsForScriptFrameworkDrift(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"fixture","devDependencies":{"vitest":"1.0.0"}}`)
	writeFile(t, filepath.Join(dir, "src", "first.test.ts"), `import { describe, it, expect, vi } from "vitest";
describe("ok", () => { it("works", () => { expect(vi.fn()).toBeDefined(); }); });
`)
	writeFile(t, filepath.Join(dir, "src", "second.test.ts"), `import { jest } from "@jest/globals";
jest.mock("./api");
test("mismatch", () => { expect(true).toBe(true); });
`)

	cfg := qualityAITestConfig(dir, "quality-ai-drift")
	cfg.Targets[0].Language = "typescript"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.local-idiom-drift")
}

func TestQualityCheckAppliesProvenancePolicy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), `package sample

// Initialize the client.
func buildClient() error {
	err := doThing()
	_ = err
	return nil
}

func doThing() error { return nil }
`)
	t.Setenv("CODEGUARD_AI_ASSISTED", "true")

	report, err := codeguard.Run(context.Background(), qualityAITestConfig(dir, "quality-ai-provenance"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.provenance-policy")
}

func TestQualityCheckPublishesChangeRiskForAIHeavyChange(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), `package sample

// Initialize the client.
func buildClient() error {
	err := doThing()
	_ = err
	return nil
}

func doThing() error { return nil }
`)
	t.Setenv("CODEGUARD_AI_ASSISTED", "true")

	report, err := codeguard.Run(context.Background(), qualityAITestConfig(dir, "quality-ai-change-risk"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.change-risk")
	for _, artifact := range report.Artifacts {
		if artifact.Kind != "change_risk" || artifact.ChangeRisk == nil {
			continue
		}
		if artifact.ChangeRisk.Score <= 0 {
			t.Fatalf("unexpected change risk artifact %#v", artifact.ChangeRisk)
		}
		return
	}
	t.Fatalf("expected change_risk artifact, got %#v", report.Artifacts)
}
