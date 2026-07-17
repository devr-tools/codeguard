package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestDesignReachabilityReportsModulesOutsideApprovedEntrypoints(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.ts"), `import "./reachable";`)
	writeFile(t, filepath.Join(dir, "src", "reachable.ts"), `export const reachable = true;`)
	writeFile(t, filepath.Join(dir, "src", "orphan.ts"), `export const orphan = true;`)

	cfg := designPolicyTestConfig(dir)
	cfg.Targets[0].Entrypoints = []string{"src/main.ts"}
	cfg.Checks.DesignRules.Reachability = &codeguard.DesignReachabilityConfig{}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	finding := findFinding(t, report, "Design Patterns", "design.unreachable-module")
	if finding.Path != "src/orphan.ts" {
		t.Fatalf("unreachable finding path = %q, want src/orphan.ts", finding.Path)
	}
}

func TestDesignReachabilityMatchesAnyFileInGoEntrypointPackage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/reachability\n\ngo 1.23.0\n")
	writeFile(t, filepath.Join(dir, "cmd", "app", "a.go"), "package main\n\nimport _ \"example.com/reachability/internal/live\"\n")
	writeFile(t, filepath.Join(dir, "cmd", "app", "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "internal", "live", "live.go"), "package live\n")
	writeFile(t, filepath.Join(dir, "internal", "orphan", "orphan.go"), "package orphan\n")

	cfg := designPolicyTestConfig(dir)
	cfg.Targets[0].Language = "go"
	cfg.Targets[0].Entrypoints = []string{"cmd/**/main.go"}
	cfg.Checks.DesignRules.Reachability = &codeguard.DesignReachabilityConfig{}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	finding := findFinding(t, report, "Design Patterns", "design.unreachable-module")
	if finding.Path != "internal/orphan/orphan.go" {
		t.Fatalf("unreachable finding path = %q, want internal/orphan/orphan.go", finding.Path)
	}
	for _, section := range report.Sections {
		for _, candidate := range section.Findings {
			if candidate.RuleID == "design.unreachable-module" && candidate.Path == "internal/live/live.go" {
				t.Fatal("module imported by the Go entrypoint package was reported unreachable")
			}
		}
	}
}

func TestDesignStabilityReportsDependencyTowardVolatileModule(t *testing.T) {
	dir := t.TempDir()
	for _, importer := range []string{"one", "two", "three"} {
		writeFile(t, filepath.Join(dir, importer+".ts"), `import "./stable";`)
	}
	writeFile(t, filepath.Join(dir, "stable.ts"), `import "./volatile";`)
	writeFile(t, filepath.Join(dir, "volatile.ts"), `
import "./leaf-one";
import "./leaf-two";
import "./leaf-three";
import "./leaf-four";
`)
	for _, leaf := range []string{"leaf-one", "leaf-two", "leaf-three", "leaf-four"} {
		writeFile(t, filepath.Join(dir, leaf+".ts"), `export const value = true;`)
	}

	cfg := designPolicyTestConfig(dir)
	cfg.Checks.DesignRules.Stability = &codeguard.DesignStabilityConfig{
		MinimumFanIn:        3,
		MaxInstabilityDelta: 0.30,
	}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	finding := findFinding(t, report, "Design Patterns", "design.stability-direction")
	if finding.Path != "stable.ts" || finding.Line != 1 {
		t.Fatalf("stability finding location = %s:%d, want stable.ts:1", finding.Path, finding.Line)
	}
}

func TestDesignGraphPoliciesCanBeDisabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.ts"), `export const main = true;`)
	writeFile(t, filepath.Join(dir, "src", "orphan.ts"), `export const orphan = true;`)
	off := false

	cfg := designPolicyTestConfig(dir)
	cfg.Checks.DesignRules.Reachability = &codeguard.DesignReachabilityConfig{
		Enabled:     &off,
		Entrypoints: []string{"src/main.ts"},
	}
	cfg.Checks.DesignRules.Stability = &codeguard.DesignStabilityConfig{Enabled: &off}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Design Patterns", "design.unreachable-module")
	assertFindingRuleAbsent(t, report, "Design Patterns", "design.stability-direction")
}

func designPolicyTestConfig(dir string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-graph-policy"
	cfg.Targets = []codeguard.TargetConfig{{Name: "app", Path: dir, Language: "typescript"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	return cfg
}
