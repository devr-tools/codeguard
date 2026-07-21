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

func TestSupplyChainFindsVulnerableDependencyFromOfflineCache(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "name": "frontend",
  "dependencies": {"example-library": "1.2.3"}
}`)
	writeFile(t, filepath.Join(dir, "package-lock.json"), `{"lockfileVersion": 3}`)
	writeFile(t, filepath.Join(dir, ".codeguard", "advisories.json"), `{
  "schema_version": 1,
  "generated_at": "2026-07-20T00:00:00Z",
  "source": "approved-export",
  "advisories": [{
    "id": "CVE-2026-12345",
    "ecosystem": "npm",
    "package": "example-library",
    "affected_versions": [">=1.0.0, <1.2.4"],
    "fixed_version": "1.2.4",
    "url": "https://example.invalid/CVE-2026-12345"
  }]
}`)

	cfg := supplyChainTestConfig(dir, "offline-advisories")
	on := true
	cfg.Checks.SupplyChainRules.DetectVulnerabilities = &on
	cfg.Checks.SupplyChainRules.AdvisoryCachePath = ".codeguard/advisories.json"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Supply Chain", "fail")
	assertFindingRulePresent(t, report, "Supply Chain", "supply_chain.vulnerable-dependency")
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID != "supply_chain.vulnerable-dependency" {
				continue
			}
			if finding.Metadata["advisory_id"] != "CVE-2026-12345" || finding.Metadata["advisory_source"] != "approved-export" {
				t.Fatalf("unexpected advisory provenance: %#v", finding.Metadata)
			}
			if finding.Metadata["cache_age"] == "" || finding.Metadata["fixed_version"] != "1.2.4" {
				t.Fatalf("missing cache metadata: %#v", finding.Metadata)
			}
			return
		}
	}
	t.Fatal("vulnerable dependency finding missing")
}

func TestSupplyChainDoesNotMatchUnresolvedDependencyRangesAgainstOfflineCache(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"dependencies": {"example-library": "^1.2.3"}}`)
	writeFile(t, filepath.Join(dir, "package-lock.json"), `{"lockfileVersion": 3}`)
	writeFile(t, filepath.Join(dir, "advisories.json"), `{"schema_version":1,"generated_at":"2026-07-20T00:00:00Z","advisories":[{"id":"CVE-2026-12345","ecosystem":"npm","package":"example-library","affected_versions":["<1.2.4"]}]}`)
	cfg := supplyChainTestConfig(dir, "unresolved-advisory-range")
	on := true
	cfg.Checks.SupplyChainRules.DetectVulnerabilities = &on
	cfg.Checks.SupplyChainRules.AdvisoryCachePath = "advisories.json"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if messages := supplyChainRuleMessages(report, "supply_chain.vulnerable-dependency"); len(messages) != 0 {
		t.Fatalf("unresolved dependency range produced advisory findings: %v", messages)
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

func TestSupplyChainParsesVCPKGBaselineAndOverrides(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "vcpkg.json"), `{
  "name": "native-app",
  "builtin-baseline": "0123456789abcdef0123456789abcdef01234567",
  "dependencies": ["fmt", {"name": "cmake", "host": true}],
  "overrides": [{"name": "fmt", "version": "10.2.1"}]
}`)

	report, err := codeguard.Run(context.Background(), supplyChainTestConfig(dir, "vcpkg-pinned"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Supply Chain", "pass")
}

func TestSupplyChainWarnsForVCPKGWithoutBaseline(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "vcpkg.json"), `{
  "name": "native-app",
  "dependencies": ["fmt", {"name": "openssl", "version>=": "3.0.0"}]
}`)

	report, err := codeguard.Run(context.Background(), supplyChainTestConfig(dir, "vcpkg-unpinned"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Supply Chain", "warn")
	messages := supplyChainRuleMessages(report, "supply_chain.unpinned-dependency")
	if len(messages) != 2 {
		t.Fatalf("unpinned vcpkg findings = %d, want 2: %v", len(messages), messages)
	}
}

func TestSupplyChainConanRequiresLockfileAndDetectsRanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "conanfile.txt"), "[requires]\nfmt/10.2.1\nopenssl/[>=3.0 <4]\n\n[tool_requires]\ncmake/3.29.0\n")

	report, err := codeguard.Run(context.Background(), supplyChainTestConfig(dir, "conan-policy"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Supply Chain", "fail")
	assertFindingRulePresent(t, report, "Supply Chain", "supply_chain.missing-lockfile")
	messages := supplyChainRuleMessages(report, "supply_chain.unpinned-dependency")
	if len(messages) != 1 || !strings.Contains(messages[0], "openssl") {
		t.Fatalf("unexpected Conan unpinned findings: %v", messages)
	}
}

func TestSupplyChainDetectsConanLockfileDrift(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "conanfile.txt"), "[requires]\nfmt/10.2.1\nopenssl/3.2.1\n")
	writeFile(t, filepath.Join(dir, "conan.lock"), `{
  "version": "0.5",
  "requires": [
    "fmt/10.2.1#recipe-revision%1700000000",
    "openssl/3.1.4#recipe-revision%1700000000"
  ]
}`)

	report, err := codeguard.Run(context.Background(), supplyChainTestConfig(dir, "conan-lock-drift"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Supply Chain", "fail")
	messages := supplyChainRuleMessages(report, "supply_chain.lockfile-drift")
	if len(messages) != 1 || !strings.Contains(messages[0], "openssl") {
		t.Fatalf("unexpected Conan lockfile drift findings: %v", messages)
	}
}
