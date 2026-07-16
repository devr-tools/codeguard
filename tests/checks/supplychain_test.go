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

func TestSupplyChainWarnsForCargoManifestWithoutLicense(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"demo\"\nversion = \"0.1.0\"\n\n[dependencies]\nserde = \"1.0.0\"\n")
	writeFile(t, filepath.Join(dir, "Cargo.lock"), "version = 3\n")

	cfg := supplyChainTestConfig(dir, "cargo-missing-license")
	off := false
	cfg.Checks.SupplyChainRules.DetectLockfileDrift = &off
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "warn")
	assertFindingRulePresent(t, report, "Supply Chain", "supply_chain.cargo.missing-package-license")
}

func TestSupplyChainWarnsForNonHermeticCargoSources(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"demo\"\nversion = \"0.1.0\"\nlicense = \"MIT\"\n\n[dependencies]\nserde = { git = \"https://github.com/serde-rs/serde\", branch = \"main\" }\nlocal = { path = \"../shared/local\" }\npinned = { git = \"https://github.com/example/pinned\", rev = \"abc123\" }\n")
	writeFile(t, filepath.Join(dir, "Cargo.lock"), "version = 3\n")

	cfg := supplyChainTestConfig(dir, "cargo-non-hermetic")
	off := false
	cfg.Checks.SupplyChainRules.DetectLockfileDrift = &off
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "warn")
	assertFindingRulePresent(t, report, "Supply Chain", "supply_chain.cargo.non-hermetic-source")
	messages := supplyChainRuleMessages(report, "supply_chain.cargo.non-hermetic-source")
	if len(messages) != 2 {
		t.Fatalf("expected 2 non-hermetic Cargo findings, got %d: %v", len(messages), messages)
	}
}
