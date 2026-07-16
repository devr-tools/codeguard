package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestSupplyChainSectionPassesWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/supplychain\n\ngo 1.23.0\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "supply-chain-enabled"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SupplyChain = true

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "pass")
}

func TestSupplyChainWarnsForUnpinnedDependencies(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "name": "frontend",
  "dependencies": {
    "react": "^18.2.0"
  },
  "devDependencies": {
    "vitest": "1.6.0"
  }
}`)
	writeFile(t, filepath.Join(dir, "package-lock.json"), `{
  "name": "frontend",
  "lockfileVersion": 3,
  "packages": {
    "": {"name": "frontend"},
    "node_modules/react": {"version": "18.2.0"},
    "node_modules/vitest": {"version": "1.6.0"}
  }
}`)

	report, err := codeguard.Run(context.Background(), supplyChainTestConfig(dir, "unpinned"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "warn")
	assertFindingRulePresent(t, report, "Supply Chain", "supply_chain.unpinned-dependency")
	if messages := supplyChainRuleMessages(report, "supply_chain.unpinned-dependency"); len(messages) != 1 || !strings.Contains(messages[0], "react") {
		t.Fatalf("unexpected unpinned messages: %v", messages)
	}
}

func TestSupplyChainFailsForMissingLockfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "name": "frontend",
  "dependencies": {
    "react": "18.2.0"
  }
}`)

	report, err := codeguard.Run(context.Background(), supplyChainTestConfig(dir, "missing-lockfile"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "fail")
	assertFindingRulePresent(t, report, "Supply Chain", "supply_chain.missing-lockfile")
}

func TestSupplyChainFailsForLockfileDriftInDiffMode(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "CodeGuard Test")
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "name": "frontend",
  "dependencies": {
    "react": "18.2.0"
  }
}`)
	writeFile(t, filepath.Join(dir, "package-lock.json"), `{
  "name": "frontend",
  "lockfileVersion": 3,
  "packages": {
    "": {"name": "frontend"},
    "node_modules/react": {"version": "18.2.0"}
  }
}`)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")

	writeFile(t, filepath.Join(dir, "package.json"), `{
  "name": "frontend",
  "dependencies": {
    "react": "18.3.0"
  }
}`)

	cfg := supplyChainTestConfig(dir, "lockfile-drift")
	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("run diff: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "fail")
	assertFindingRulePresent(t, report, "Supply Chain", "supply_chain.lockfile-drift")
}
