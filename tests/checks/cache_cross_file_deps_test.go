package checks_test

import (
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// The per-file findings cache is keyed on the scanned file's own content, but
// several evaluators read state from other files: the Go quality pass builds a
// package-wide symbol index from sibling files and resolves module paths from
// go.mod, and the Go design pass resolves the SDK facade from go.mod and pkg/.
// Editing only one of those inputs must still change the findings reported
// for an unchanged file, or a cached scan silently replays stale results.

func cachedScanConfig(t *testing.T, cfg codeguard.Config) codeguard.Config {
	t.Helper()
	on := true
	cfg.Cache.Enabled = &on
	cfg.Cache.Path = filepath.Join(t.TempDir(), "cache.json")
	return cfg
}

func findingInFile(report codeguard.Report, section string, ruleID string, path string) bool {
	for _, result := range report.Sections {
		if result.Name != section {
			continue
		}
		for _, finding := range result.Findings {
			if finding.RuleID == ruleID && filepath.ToSlash(finding.Path) == path {
				return true
			}
		}
	}
	return false
}

// assertCachedMatchesUncached runs a fresh uncached scan as the ground truth
// for the edited tree, so the regression cannot pass merely because the edit
// failed to change the rule's verdict.
func assertCachedMatchesUncached(t *testing.T, cached codeguard.Report, uncachedCfg codeguard.Config, section, ruleID, path string, want bool) {
	t.Helper()
	uncached := runQualityPrecisionScan(t, uncachedCfg)
	if got := findingInFile(uncached, section, ruleID, path); got != want {
		t.Fatalf("fixture invalid: uncached scan reports %s in %s = %t, want %t", ruleID, path, got, want)
	}
	if got := findingInFile(cached, section, ruleID, path); got != want {
		t.Fatalf("cached scan reports %s in unchanged %s = %t, want %t (stale per-file cache entry replayed)", ruleID, path, got, want)
	}
}

func TestCacheInvalidatesWhenSiblingPackageFileChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "state.go"), "package sample\n\ntype State struct{ Value int }\n\nvar sharedState *State\n")
	writeFile(t, filepath.Join(dir, "current.go"), "package sample\n\nfunc CurrentGlobal() int { sharedState.Value++; return sharedState.Value }\n")

	uncachedCfg := qualityPrecisionConfig(dir)
	cfg := cachedScanConfig(t, uncachedCfg)

	first := runQualityPrecisionScan(t, cfg)
	if !findingInFile(first, "Code Quality", "function.hidden-mutation", "current.go") {
		t.Fatal("first scan: expected function.hidden-mutation in current.go via the cross-file package global")
	}

	// Only the sibling changes: the package global disappears, so the mutation
	// in current.go no longer resolves to shared state.
	writeFile(t, filepath.Join(dir, "state.go"), "package sample\n\ntype State struct{ Value int }\n")

	second := runQualityPrecisionScan(t, cfg)
	assertCachedMatchesUncached(t, second, uncachedCfg, "Code Quality", "function.hidden-mutation", "current.go", false)
}

func TestCacheInvalidatesQualityWhenGoModChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/m\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "a", "a.go"), "package a\n\nimport _ \"example.com/m/internal/x\"\n")

	uncachedCfg := qualityPrecisionConfig(dir)
	cfg := cachedScanConfig(t, uncachedCfg)

	first := runQualityPrecisionScan(t, cfg)
	if findingInFile(first, "Code Quality", "quality.dependency-direction", "a/a.go") {
		t.Fatal("first scan: a same-module internal import must not be reported")
	}

	// Renaming the module moves a/ out of the internal tree's scope.
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/other\n\ngo 1.22\n")

	second := runQualityPrecisionScan(t, cfg)
	assertCachedMatchesUncached(t, second, uncachedCfg, "Code Quality", "quality.dependency-direction", "a/a.go", true)
}

func TestCacheInvalidatesDesignWhenSDKFacadeModuleChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/m\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "pkg", "sdk", "sdk.go"), "package sdk\n\nfunc Run() {}\n")
	writeFile(t, filepath.Join(dir, "cmd", "app", "main.go"), "package main\n\nimport \"example.com/m/pkg/sdk\"\n\nfunc main() { sdk.Run() }\n")

	uncachedCfg := qualityPrecisionConfig(dir)
	uncachedCfg.Checks.Quality = false
	uncachedCfg.Checks.Design = true
	cfg := cachedScanConfig(t, uncachedCfg)

	first := runQualityPrecisionScan(t, cfg)
	if !findingInFile(first, "Design Patterns", "design.cmd-through-internal-cli", "cmd/app/main.go") {
		t.Fatal("first scan: expected design.cmd-through-internal-cli for a cmd/ import of the facade")
	}

	// The facade's module path comes from go.mod: after the rename the import
	// no longer refers to this repository's facade.
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/other\n\ngo 1.22\n")

	second := runQualityPrecisionScan(t, cfg)
	assertCachedMatchesUncached(t, second, uncachedCfg, "Design Patterns", "design.cmd-through-internal-cli", "cmd/app/main.go", false)
}

// Two design passes scan the same Go files: the per-file Go rules and the
// language-neutral local-abstraction rules. They must not share a cache
// entry, or the second pass replays the first pass's findings in its place.
func TestCacheKeepsDesignPassesSeparate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pkg", "api", "order_handler.go"), "package api\n\nfunc HandleOrder(order Order) {\n\tif order.Status == \"paid\" {\n\t\trepo.Save(order)\n\t}\n\tif order.Amount > 1000 {\n\t\trepo.Update(order)\n\t}\n}\n")

	uncachedCfg := designLocalConfig(dir, "go")
	cfg := cachedScanConfig(t, uncachedCfg)
	for run := 1; run <= 2; run++ {
		report := runDesignLocalScan(t, cfg)
		if !findingInFile(report, "Design Patterns", "design.domain-logic-in-handler", "pkg/api/order_handler.go") {
			t.Fatalf("cached run %d: design.domain-logic-in-handler missing; a colliding cache entry replaced the local-abstraction findings", run)
		}
	}
	assertCachedMatchesUncached(t, runDesignLocalScan(t, cfg), uncachedCfg, "Design Patterns", "design.domain-logic-in-handler", "pkg/api/order_handler.go", true)
}
