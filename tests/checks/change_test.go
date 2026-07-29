package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func changeSafetyTestConfig(name string, dir string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SupplyChain = false
	on := true
	off := false
	cfg.Checks.Reliability = &off
	cfg.Checks.Data = &off
	cfg.Checks.Change = &on
	cfg.Checks.Contracts = &off
	cfg.Checks.Context = &off
	cfg.Cache.Enabled = &off
	return cfg
}

func runChangeDiff(t *testing.T, cfg codeguard.Config) codeguard.Report {
	t.Helper()
	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("change diff scan: %v", err)
	}
	return report
}

func initChangeRepo(t *testing.T) string {
	t.Helper()
	return initContractsRepo(t)
}

func TestChangeOversizedDiffUsesConfiguredThresholds(t *testing.T) {
	dir := initChangeRepo(t)
	for _, rel := range []string{
		"service/a.go",
		"service/b.go",
		"workers/c.go",
	} {
		writeFile(t, filepath.Join(dir, rel), "package sample\n\nfunc Value() int { return 1 }\n")
	}
	commitAll(t, dir, "base")

	for _, rel := range []string{
		"service/a.go",
		"service/b.go",
		"workers/c.go",
	} {
		writeFile(t, filepath.Join(dir, rel), "package sample\n\nfunc Value() int {\n\treturn 2\n}\n")
	}

	cfg := changeSafetyTestConfig("change-oversized", dir)
	cfg.Checks.ChangeRules.MaxChangedFiles = 2
	cfg.Checks.ChangeRules.MaxChangedDirectories = 1
	cfg.Checks.ChangeRules.MaxChangedLines = 2
	cfg.Checks.ChangeRules.MinTestToProductionRatioPercent = 50

	report := runChangeDiff(t, cfg)
	assertFindingRulePresent(t, report, "Change Safety", "change.oversized-diff")
	finding := changeRuleFinding(t, report, "change.oversized-diff")
	if finding.Metadata["files_touched"] != "3" {
		t.Fatalf("files_touched metadata = %q, want 3", finding.Metadata["files_touched"])
	}
	if finding.Metadata["directories_touched"] != "2" {
		t.Fatalf("directories_touched metadata = %q, want 2", finding.Metadata["directories_touched"])
	}
	if finding.Metadata["test_to_production_ratio_percent"] != "0" {
		t.Fatalf("ratio metadata = %q, want 0", finding.Metadata["test_to_production_ratio_percent"])
	}
}

func TestChangeFocusedDiffWithTestsPasses(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "service", "price.go"), "package service\n\nfunc Price() int { return 1 }\n")
	writeFile(t, filepath.Join(dir, "service", "price_test.go"), "package service\n\nfunc TestPrice(t *testing.T) {}\n")
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "service", "price.go"), "package service\n\nfunc Price() int { return 2 }\n")
	writeFile(t, filepath.Join(dir, "service", "price_test.go"), "package service\n\nfunc TestPrice(t *testing.T) { if Price() != 2 { t.Fatal() } }\n")

	report := runChangeDiff(t, changeSafetyTestConfig("change-focused", dir))
	assertSectionStatus(t, report, "Change Safety", "pass")
}

func TestChangeDetectsMixedAndTooManyConcerns(t *testing.T) {
	dir := initChangeRepo(t)
	fixtures := map[string]string{
		"api/handler.go":  "package api\n\nfunc Handle() int { return 1 }\n",
		"db/store.go":     "package db\n\nfunc Store() int { return 1 }\n",
		"ui/view.tsx":     "export function View() { return 1 }\n",
		"infra/deploy.go": "package infra\n\nfunc Deploy() int { return 1 }\n",
	}
	for rel, content := range fixtures {
		writeFile(t, filepath.Join(dir, rel), content)
	}
	commitAll(t, dir, "base")
	for rel, content := range fixtures {
		writeFile(t, filepath.Join(dir, rel), strings.Replace(content, "return 1", "return 2", 1))
	}

	cfg := changeSafetyTestConfig("change-concerns", dir)
	cfg.Checks.ChangeRules.MaxChangedFiles = 20
	cfg.Checks.ChangeRules.MaxChangedDirectories = 20
	cfg.Checks.ChangeRules.MaxChangedLines = 100
	cfg.Checks.ChangeRules.MaxPublicInterfacesChanged = 20
	cfg.Checks.ChangeRules.MaxConcernFamilies = 2
	cfg.Checks.ChangeRules.MinTestToProductionRatioPercent = 0

	report := runChangeDiff(t, cfg)
	assertFindingRulePresent(t, report, "Change Safety", "change.mixed-concerns")
	assertFindingRulePresent(t, report, "Change Safety", "change.too-many-concerns")
}

func TestChangeDetectsMoveMixedWithBehaviorAndNoVerification(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "service", "handler.go"), "package service\n\nfunc Handle() int {\n\treturn 1\n}\n")
	commitAll(t, dir, "base")

	if err := os.Remove(filepath.Join(dir, "service", "handler.go")); err != nil {
		t.Fatalf("remove old file: %v", err)
	}
	writeFile(t, filepath.Join(dir, "app", "handler.go"), "package app\n\nfunc Handle() int {\n\tif true {\n\t\treturn 2\n\t}\n\treturn 1\n}\n")
	runGit(t, dir, "add", "-N", "app/handler.go")

	cfg := changeSafetyTestConfig("change-move", dir)
	cfg.Checks.ChangeRules.MaxChangedFiles = 20
	cfg.Checks.ChangeRules.MaxChangedDirectories = 20
	cfg.Checks.ChangeRules.MaxChangedLines = 100
	cfg.Checks.ChangeRules.MaxPublicInterfacesChanged = 20
	cfg.Checks.ChangeRules.MaxConcernFamilies = 20

	report := runChangeDiff(t, cfg)
	assertFindingRulePresent(t, report, "Change Safety", "change.mixed-refactor-and-behavior")
	assertFindingRulePresent(t, report, "Change Safety", "change.move-without-verification")
}

func TestChangeMoveWithVerificationDoesNotWarnAboutMissingVerification(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "service", "worker.go"), "package service\n\nfunc Work() int { return 1 }\n")
	commitAll(t, dir, "base")

	if err := os.Remove(filepath.Join(dir, "service", "worker.go")); err != nil {
		t.Fatalf("remove old file: %v", err)
	}
	writeFile(t, filepath.Join(dir, "app", "worker.go"), "package app\n\nfunc Work() int { return 1 }\n")
	writeFile(t, filepath.Join(dir, "app", "worker_test.go"), "package app\n\nfunc TestWork(t *testing.T) { if Work() != 1 { t.Fatal() } }\n")
	runGit(t, dir, "add", "-N", "app/worker.go", "app/worker_test.go")

	report := runChangeDiff(t, changeSafetyTestConfig("change-move-verified", dir))
	assertFindingRuleAbsent(t, report, "Change Safety", "change.move-without-verification")
}

func TestChangeDetectsUnnecessarySurfaceArea(t *testing.T) {
	dir := initChangeRepo(t)
	fixtures := map[string]string{
		"pkg/client/api.go":       "package client\n\nfunc Do() int { return 1 }\n",
		"include/demo/client.hpp": "#pragma once\nint Do();\n",
		"api/openapi.yaml":        "openapi: 3.0.0\ninfo: {title: demo, version: '1'}\n",
		"proto/service.proto":     "syntax = \"proto3\";\nmessage Request {}\n",
	}
	for rel, content := range fixtures {
		writeFile(t, filepath.Join(dir, rel), content)
	}
	commitAll(t, dir, "base")
	for rel, content := range fixtures {
		writeFile(t, filepath.Join(dir, rel), content+"\n")
	}

	cfg := changeSafetyTestConfig("change-surface", dir)
	cfg.Checks.ChangeRules.MaxChangedFiles = 20
	cfg.Checks.ChangeRules.MaxChangedDirectories = 20
	cfg.Checks.ChangeRules.MaxChangedLines = 100
	cfg.Checks.ChangeRules.MaxPublicInterfacesChanged = 2
	cfg.Checks.ChangeRules.MaxConcernFamilies = 20

	report := runChangeDiff(t, cfg)
	assertFindingRulePresent(t, report, "Change Safety", "change.unnecessary-surface-area")
}

func TestChangeFullScanNoops(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\nfunc Value() int { return 1 }\n")

	report, err := codeguard.Run(context.Background(), changeSafetyTestConfig("change-full", dir))
	if err != nil {
		t.Fatalf("full scan: %v", err)
	}
	assertSectionStatus(t, report, "Change Safety", "pass")
}

func changeRuleFinding(t *testing.T, report codeguard.Report, ruleID string) codeguard.Finding {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name != "Change Safety" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				return finding
			}
		}
	}
	t.Fatalf("missing change finding %q", ruleID)
	return codeguard.Finding{}
}
