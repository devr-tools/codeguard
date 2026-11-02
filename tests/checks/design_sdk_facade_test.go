package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// facadeFixture writes a minimal single-module Go repository whose pkg/<name>
// package imports its own internal/ tree - the legal, idiomatic facade
// pattern that used to be reported at fail level for every repository whose
// facade directory was not literally named "codeguard".
func facadeFixture(t *testing.T, name string, module string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module "+module+"\n\ngo 1.23\n")
	writeFile(t, filepath.Join(dir, "internal", "engine", "engine.go"),
		"package engine\n\nfunc Run() string { return \"ok\" }\n")
	writeFile(t, filepath.Join(dir, "pkg", name, "facade.go"),
		"package "+name+"\n\nimport \""+module+"/internal/engine\"\n\nfunc Run() string { return engine.Run() }\n")
	return dir
}

func designLayeringConfig(t *testing.T, name string, dir string) codeguard.Config {
	t.Helper()
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	return cfg
}

func runDesignScan(t *testing.T, cfg codeguard.Config) codeguard.Report {
	t.Helper()
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func assertNoFindings(t *testing.T, report codeguard.Report, ruleID string) {
	t.Helper()
	if found := findingsForRule(report, ruleID); len(found) > 0 {
		t.Fatalf("%s reported %d finding(s), want none: %+v", ruleID, len(found), found)
	}
}

func assertHasFindings(t *testing.T, report codeguard.Report, ruleID string) {
	t.Helper()
	if found := findingsForRule(report, ruleID); len(found) == 0 {
		t.Fatalf("%s reported no findings, want at least one", ruleID)
	}
}

// TestDesignServiceImportInternalExemptsDiscoveredFacade is the fixture that
// proved the defect: an ordinary repository whose facade directory happens not
// to be named "codeguard" used to collect a fail-level
// design.service-import-internal for a legal same-module import.
func TestDesignServiceImportInternalExemptsDiscoveredFacade(t *testing.T) {
	dir := facadeFixture(t, "myapp", "example.com/myapp")
	cfg := designLayeringConfig(t, "facade-ordinary-repo", dir)
	// quality.dependency-direction consumed the same predicate, so it is
	// asserted here too; that needs the quality section on.
	cfg.Checks.Quality = true
	report := runDesignScan(t, cfg)
	assertNoFindings(t, report, "design.service-import-internal")
	assertNoFindings(t, report, "quality.dependency-direction")
}

// TestDesignServiceImportInternalExemptsCodeguardLayout pins that generalising
// the predicate did not change this repository's own result.
func TestDesignServiceImportInternalExemptsCodeguardLayout(t *testing.T) {
	dir := facadeFixture(t, "codeguard", "github.com/devr-tools/codeguard")
	report := runDesignScan(t, designLayeringConfig(t, "facade-codeguard-layout", dir))
	assertNoFindings(t, report, "design.service-import-internal")
}

// TestDesignServiceImportInternalFiresForNonFacadePackage keeps the rule alive:
// once a repository states which package is its facade, every other public
// package reaching into internal/ is still a fail.
func TestDesignServiceImportInternalFiresForNonFacadePackage(t *testing.T) {
	dir := facadeFixture(t, "myapp", "example.com/myapp")
	writeFile(t, filepath.Join(dir, "pkg", "widgets", "widgets.go"),
		"package widgets\n\nimport \"example.com/myapp/internal/engine\"\n\nfunc Run() string { return engine.Run() }\n")

	cfg := designLayeringConfig(t, "facade-configured", dir)
	cfg.Checks.DesignRules.SDKFacadePaths = []string{"pkg/myapp"}
	report := runDesignScan(t, cfg)

	assertHasFindings(t, report, "design.service-import-internal")
	for _, finding := range findingsForRule(report, "design.service-import-internal") {
		if filepath.ToSlash(finding.Path) != "pkg/widgets/widgets.go" {
			t.Fatalf("unexpected finding path %q; the configured facade must stay exempt", finding.Path)
		}
	}
}

// TestDesignServiceImportCmdHasNoFacadeExemption documents the deliberate
// asymmetry: a public package importing cmd/ is a layering inversion under any
// layout, so the facade exemption does not cover it.
func TestDesignServiceImportCmdHasNoFacadeExemption(t *testing.T) {
	dir := facadeFixture(t, "myapp", "example.com/myapp")
	writeFile(t, filepath.Join(dir, "cmd", "myapp", "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "pkg", "myapp", "shell.go"),
		"package myapp\n\nimport _ \"example.com/myapp/cmd/myapp\"\n")

	report := runDesignScan(t, designLayeringConfig(t, "facade-import-cmd", dir))
	assertHasFindings(t, report, "design.service-import-cmd")
}

// TestDesignCmdThroughInternalCLIFiresOutsideCodeguard covers the inverse
// defect: the rule required "/pkg/codeguard" in the imported path, so it was a
// guaranteed no-op for every other repository.
func TestDesignCmdThroughInternalCLIFiresOutsideCodeguard(t *testing.T) {
	dir := facadeFixture(t, "myapp", "example.com/myapp")
	writeFile(t, filepath.Join(dir, "cmd", "myapp", "main.go"),
		"package main\n\nimport \"example.com/myapp/pkg/myapp\"\n\nfunc main() { println(myapp.Run()) }\n")

	report := runDesignScan(t, designLayeringConfig(t, "cmd-through-cli-ordinary-repo", dir))
	assertHasFindings(t, report, "design.cmd-through-internal-cli")
}

// TestDesignCmdThroughInternalCLIIgnoresForeignFacade pins the module
// qualification: a third party's pkg/<same name> is not this repository's
// facade.
func TestDesignCmdThroughInternalCLIIgnoresForeignFacade(t *testing.T) {
	dir := facadeFixture(t, "myapp", "example.com/myapp")
	writeFile(t, filepath.Join(dir, "cmd", "myapp", "main.go"),
		"package main\n\nimport _ \"github.com/someone/else/pkg/myapp\"\n\nfunc main() {}\n")

	report := runDesignScan(t, designLayeringConfig(t, "cmd-through-cli-foreign", dir))
	assertNoFindings(t, report, "design.cmd-through-internal-cli")
}
