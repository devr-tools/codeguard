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

func TestQualityCheckResolvesTypeScriptPNPMMonorepoImports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"repo","private":true}`)
	writeFile(t, filepath.Join(dir, "pnpm-workspace.yaml"), "packages:\n  - packages/*\n")
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "lockfileVersion: '9.0'\n\npackages:\n\n  lock-only-package@1.0.0:\n    resolution: {integrity: sha512-example}\n")
	writeFile(t, filepath.Join(dir, "packages", "app", "package.json"), `{
  "name":"@legal-nest/app",
  "dependencies":{"react":"18.0.0","next":"15.0.0","@prisma/client":"6.0.0"}
}`)
	writeFile(t, filepath.Join(dir, "packages", "shared", "package.json"), `{"name":"@legal-nest/shared"}`)
	writeFile(t, filepath.Join(dir, "packages", "app", "tsconfig.json"), `{
  // aliases are valid JSONC in tsconfig files
  "compilerOptions": {"baseUrl":".", "paths":{"app/*":["src/*"]}}
}`)
	writeFile(t, filepath.Join(dir, "packages", "app", "src", "config.ts"), "export const config = {};\n")
	writeFile(t, filepath.Join(dir, "packages", "app", "src", "lib", "value.ts"), "export const value = 1;\n")
	writeFile(t, filepath.Join(dir, "node_modules", ".pnpm", "installed-package@1.0.0", "node_modules", "installed-package", "package.json"), `{"name":"installed-package"}`)
	writeFile(t, filepath.Join(dir, "packages", "app", "src", "app.ts"), `
import { useState } from "react";
import { useRouter } from "next/navigation";
import { prisma } from "@prisma/client";
import { config } from "./config.js";
import { value } from "app/lib/value";
import { shared } from "@legal-nest/shared";
import installed from "installed-package";
import lockOnly from "lock-only-package";
void useState; void useRouter; void prisma; void config; void value; void shared; void installed; void lockOnly;
`)

	cfg := qualityAITestConfig(dir, "quality-ai-ts-pnpm-monorepo")
	cfg.Targets[0].Language = "typescript"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.ai.hallucinated-import")
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
