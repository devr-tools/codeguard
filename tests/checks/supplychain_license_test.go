package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestSupplyChainFailsForDeniedManifestLicense(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), `[package]
name = "worker"
version = "0.1.0"
license = "GPL-3.0"

[dependencies]
tokio = "=1.38.0"
`)
	writeFile(t, filepath.Join(dir, "Cargo.lock"), "# lock\n")

	cfg := supplyChainTestConfig(dir, "denied-license")
	cfg.Checks.SupplyChainRules.DeniedLicenses = []string{"GPL-3.0"}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "fail")
	assertFindingRulePresent(t, report, "Supply Chain", "supply_chain.denied-license")
}

func TestSupplyChainFailsForDeniedDependencyLicenseFromNodeModules(t *testing.T) {
	dir := t.TempDir()
	writeNodeLicenseFixture(t, dir, nodeLicenseFixture{
		packagePath:    "package.json",
		lockfilePath:   "package-lock.json",
		nodeModulesDir: "node_modules",
		license:        "GPL-3.0",
	})

	cfg := supplyChainTestConfig(dir, "denied-dependency-license")
	cfg.Checks.SupplyChainRules.DeniedLicenses = []string{"GPL-3.0"}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "fail")
	assertFindingRulePresent(t, report, "Supply Chain", "supply_chain.denied-license")
	if messages := supplyChainRuleMessages(report, "supply_chain.denied-license"); len(messages) == 0 || !strings.Contains(messages[0], "left-pad") {
		t.Fatalf("unexpected denied dependency license messages: %v", messages)
	}
}

func TestSupplyChainResolvesNestedManifestDependencyLicense(t *testing.T) {
	dir := t.TempDir()
	writeNodeLicenseFixture(t, dir, nodeLicenseFixture{
		packagePath:    filepath.Join("web", "package.json"),
		lockfilePath:   filepath.Join("web", "package-lock.json"),
		nodeModulesDir: filepath.Join("web", "node_modules"),
		license:        "GPL-3.0",
	})

	cfg := supplyChainTestConfig(dir, "nested-node-license")
	cfg.Checks.SupplyChainRules.DeniedLicenses = []string{"GPL-3.0"}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "fail")
	if messages := supplyChainRuleMessages(report, "supply_chain.denied-license"); len(messages) == 0 || !strings.Contains(messages[0], "left-pad") {
		t.Fatalf("unexpected nested dependency license messages: %v", messages)
	}
}

func TestSupplyChainAllowsMultiLicenseExpressionWhenAllTermsAllowed(t *testing.T) {
	dir := t.TempDir()
	writeNodeLicenseFixture(t, dir, nodeLicenseFixture{
		packagePath:    "package.json",
		lockfilePath:   "package-lock.json",
		nodeModulesDir: "node_modules",
		license:        "MIT OR Apache-2.0",
	})

	cfg := supplyChainTestConfig(dir, "multi-license-allowed")
	cfg.Checks.SupplyChainRules.AllowedLicenses = []string{"MIT", "Apache-2.0"}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "pass")
	if messages := supplyChainRuleMessages(report, "supply_chain.denied-license"); len(messages) != 0 {
		t.Fatalf("expected no denied license finding, got %v", messages)
	}
}

func TestSupplyChainResolvesDependencyLicenseFromCommand(t *testing.T) {
	dir := t.TempDir()
	writeNodeManifestOnlyFixture(t, dir)
	script := filepath.Join(dir, "resolve-licenses.sh")
	writeExecutableFile(t, script, "#!/bin/sh\nset -eu\n[ \"$CODEGUARD_SUPPLY_CHAIN_ECOSYSTEM\" = \"npm\" ]\n[ \"$CODEGUARD_SUPPLY_CHAIN_MANIFEST_PATH\" = \"package.json\" ]\n[ \"$CODEGUARD_SUPPLY_CHAIN_TARGET_NAME\" = \"repo\" ]\n[ \"$CODEGUARD_SUPPLY_CHAIN_UNRESOLVED_NAMES\" = \"left-pad\" ]\n[ \"$CODEGUARD_SUPPLY_CHAIN_UNRESOLVED_COORDINATES\" = \"left-pad@1.3.0\" ]\n[ -f \"$CODEGUARD_SUPPLY_CHAIN_CONTEXT_FILE\" ]\ngrep -q '\"ecosystem\":\"npm\"' \"$CODEGUARD_SUPPLY_CHAIN_CONTEXT_FILE\"\ngrep -q '\"manifest_path\":\"package.json\"' \"$CODEGUARD_SUPPLY_CHAIN_CONTEXT_FILE\"\ngrep -q '\"name\":\"left-pad\"' \"$CODEGUARD_SUPPLY_CHAIN_CONTEXT_FILE\"\ngrep -q '\"coordinate\":\"left-pad@1.3.0\"' \"$CODEGUARD_SUPPLY_CHAIN_CONTEXT_FILE\"\nprintf '%s\n' '[{\"coordinate\":\"left-pad@1.3.0\",\"license\":\"GPL-3.0\",\"source\":\"license-command\"}]'\n")

	cfg := supplyChainTestConfig(dir, "command-license")
	cfg.Checks.SupplyChainRules.DeniedLicenses = []string{"GPL-3.0"}
	cfg.Checks.SupplyChainRules.LicenseCommands = map[string]codeguard.CommandCheckConfig{
		"npm": {Name: "npm-license-resolver", Command: script},
	}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "fail")
	assertFindingRulePresent(t, report, "Supply Chain", "supply_chain.denied-license")
	if messages := supplyChainRuleMessages(report, "supply_chain.denied-license"); len(messages) == 0 || !strings.Contains(messages[0], "license-command") {
		t.Fatalf("unexpected command-backed license messages: %v", messages)
	}
	deps := requireSupplyChainArtifact(t, report, 1).SupplyChain.Manifests[0].Dependencies
	if len(deps) != 1 || deps[0].License != "GPL-3.0" || deps[0].LicenseSource != "license-command" {
		t.Fatalf("unexpected command-backed dependency metadata: %#v", deps)
	}
}

func TestSupplyChainPrefersDefinitiveCommandLicenseCandidate(t *testing.T) {
	dir := t.TempDir()
	writeNodeManifestOnlyFixture(t, dir)
	script := filepath.Join(dir, "resolve-licenses.sh")
	writeExecutableFile(t, script, "#!/bin/sh\nset -eu\nprintf '%s\n' '[{\"coordinate\":\"left-pad@1.3.0\",\"candidates\":[{\"license\":\"GPL-3.0\",\"confidence\":\"low\",\"provenance\":\"heuristic-text-match\",\"source\":\"license-command\"},{\"license\":\"MIT\",\"confidence\":\"high\",\"provenance\":\"spdx-expression\",\"source\":\"license-command\"}]}]'\n")

	cfg := supplyChainTestConfig(dir, "candidate-license")
	cfg.Checks.SupplyChainRules.DeniedLicenses = []string{"GPL-3.0"}
	cfg.Checks.SupplyChainRules.LicenseCommands = map[string]codeguard.CommandCheckConfig{
		"npm": {Name: "npm-license-resolver", Command: script},
	}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "pass")
	if messages := supplyChainRuleMessages(report, "supply_chain.denied-license"); len(messages) != 0 {
		t.Fatalf("expected no denied license findings, got %v", messages)
	}
	deps := requireSupplyChainArtifact(t, report, 1).SupplyChain.Manifests[0].Dependencies
	if len(deps) != 1 {
		t.Fatalf("dependency count = %d, want 1", len(deps))
	}
	if deps[0].License != "MIT" {
		t.Fatalf("selected license = %q, want MIT", deps[0].License)
	}
	if len(deps[0].LicenseCandidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(deps[0].LicenseCandidates))
	}
	if deps[0].LicenseCandidates[0].License != "MIT" {
		t.Fatalf("best candidate = %q, want MIT", deps[0].LicenseCandidates[0].License)
	}
}

func TestSupplyChainWarnsForHeuristicCommandLicenseCandidate(t *testing.T) {
	dir := t.TempDir()
	writeNodeManifestOnlyFixture(t, dir)
	script := filepath.Join(dir, "resolve-licenses.sh")
	writeExecutableFile(t, script, "#!/bin/sh\nset -eu\nprintf '%s\n' '[{\"coordinate\":\"left-pad@1.3.0\",\"candidates\":[{\"license\":\"GPL-3.0\",\"confidence\":\"low\",\"provenance\":\"heuristic-text-match\",\"source\":\"license-command\"}]}]'\n")

	cfg := supplyChainTestConfig(dir, "heuristic-license")
	cfg.Checks.SupplyChainRules.DeniedLicenses = []string{"GPL-3.0"}
	cfg.Checks.SupplyChainRules.LicenseCommands = map[string]codeguard.CommandCheckConfig{
		"npm": {Name: "npm-license-resolver", Command: script},
	}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "warn")
	assertFindingRulePresent(t, report, "Supply Chain", "supply_chain.denied-license")
	messages := supplyChainRuleMessages(report, "supply_chain.denied-license")
	if len(messages) == 0 || !strings.Contains(messages[0], "confidence=low") || !strings.Contains(messages[0], "heuristic") {
		t.Fatalf("unexpected heuristic license messages: %v", messages)
	}
}

func writeNodeManifestOnlyFixture(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "name": "frontend",
  "dependencies": {
    "left-pad": "1.3.0"
  }
}`)
	writeFile(t, filepath.Join(dir, "package-lock.json"), `{
  "name": "frontend",
  "lockfileVersion": 3,
  "packages": {
    "": {"name": "frontend"},
    "node_modules/left-pad": {"version": "1.3.0"}
  }
}`)
}

type nodeLicenseFixture struct {
	packagePath    string
	lockfilePath   string
	nodeModulesDir string
	license        string
}

func writeNodeLicenseFixture(t *testing.T, dir string, fixture nodeLicenseFixture) {
	t.Helper()
	writeNodeManifestOnlyFixture(t, dirForManifest(dir, fixture.packagePath))
	if fixture.packagePath != "package.json" {
		writeFile(t, filepath.Join(dir, fixture.packagePath), `{
  "name": "frontend",
  "dependencies": {
    "left-pad": "1.3.0"
  }
}`)
		writeFile(t, filepath.Join(dir, fixture.lockfilePath), `{
  "name": "frontend",
  "lockfileVersion": 3,
  "packages": {
    "": {"name": "frontend"},
    "node_modules/left-pad": {"version": "1.3.0"}
  }
}`)
	}
	writeFile(t, filepath.Join(dir, fixture.nodeModulesDir, "left-pad", "package.json"), `{
  "name": "left-pad",
  "version": "1.3.0",
  "license": "`+fixture.license+`"
}`)
}

func dirForManifest(root string, packagePath string) string {
	if packagePath == "package.json" {
		return root
	}
	return filepath.Join(root, filepath.Dir(packagePath))
}
