package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestSupplyChainResolvesDependencyLicenseByCoordinate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "leftpad==1.0.0\nleftpad==2.0.0\n")
	script := filepath.Join(dir, "resolve-licenses.sh")
	writeExecutableFile(t, script, "#!/bin/sh\nset -eu\n[ \"$CODEGUARD_SUPPLY_CHAIN_ECOSYSTEM\" = \"python\" ]\n[ \"$CODEGUARD_SUPPLY_CHAIN_UNRESOLVED_COORDINATES\" = \"leftpad@1.0.0,leftpad@2.0.0\" ]\ngrep -q '\"coordinate\":\"leftpad@1.0.0\"' \"$CODEGUARD_SUPPLY_CHAIN_CONTEXT_FILE\"\ngrep -q '\"coordinate\":\"leftpad@2.0.0\"' \"$CODEGUARD_SUPPLY_CHAIN_CONTEXT_FILE\"\nprintf '%s\n' '[{\"coordinate\":\"leftpad@1.0.0\",\"license\":\"MIT\",\"source\":\"license-command\"},{\"coordinate\":\"leftpad@2.0.0\",\"license\":\"Apache-2.0\",\"source\":\"license-command\"}]'\n")

	cfg := supplyChainTestConfig(dir, "coordinate-license")
	cfg.Checks.SupplyChainRules.LicenseCommands = map[string]codeguard.CommandCheckConfig{
		"python": {Name: "python-license-resolver", Command: script},
	}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "pass")
	deps := requireSupplyChainArtifact(t, report, 1).SupplyChain.Manifests[0].Dependencies
	if len(deps) != 2 {
		t.Fatalf("dependency count = %d, want 2", len(deps))
	}
	licensesByVersion := map[string]string{}
	for _, dep := range deps {
		licensesByVersion[dep.Version] = dep.License
	}
	if licensesByVersion["1.0.0"] != "MIT" {
		t.Fatalf("version 1.0.0 license = %q, want MIT", licensesByVersion["1.0.0"])
	}
	if licensesByVersion["2.0.0"] != "Apache-2.0" {
		t.Fatalf("version 2.0.0 license = %q, want Apache-2.0", licensesByVersion["2.0.0"])
	}
}

func TestSupplyChainValidatesBunLockfileContent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "name": "frontend",
  "packageManager": "bun@1.1.0",
  "dependencies": {
    "left-pad": "1.3.0"
  }
}`)
	writeFile(t, filepath.Join(dir, "bun.lock"), `{
  "packages": {
    "left-pad@1.3.0": {}
  }
}`)

	report, err := codeguard.Run(context.Background(), supplyChainTestConfig(dir, "bun-lock"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "pass")

	writeFile(t, filepath.Join(dir, "bun.lock"), `{
  "packages": {
    "other@1.0.0": {}
  }
}`)

	report, err = codeguard.Run(context.Background(), supplyChainTestConfig(dir, "bun-lock-stale"))
	if err != nil {
		t.Fatalf("run stale: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "fail")
	assertFindingRulePresent(t, report, "Supply Chain", "supply_chain.lockfile-drift")
}

func TestSupplyChainFailsForStaleGoSumInFullScan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/stale\n\ngo 1.23.0\n\nrequire github.com/stretchr/testify v1.9.0\n")
	writeFile(t, filepath.Join(dir, "go.sum"), "github.com/other/module v1.0.0 h1:test\n")

	report, err := codeguard.Run(context.Background(), supplyChainTestConfig(dir, "stale-go-sum"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "fail")
	assertFindingRulePresent(t, report, "Supply Chain", "supply_chain.lockfile-drift")
	if messages := supplyChainRuleMessages(report, "supply_chain.lockfile-drift"); len(messages) == 0 || !strings.Contains(messages[0], "github.com/stretchr/testify") {
		t.Fatalf("unexpected lockfile drift messages: %v", messages)
	}
}
