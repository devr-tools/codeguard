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

func TestDesignReachabilityFollowsWorkspacePackageExportSubpaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.ts"), `import "@repo/shared/token";`)
	writeFile(t, filepath.Join(dir, "packages", "shared", "package.json"), "{\n  \"name\": \"@repo/shared\",\n  \"exports\": {\n    \"./token\": \"./src/token.ts\"\n  }\n}\n")
	writeFile(t, filepath.Join(dir, "packages", "shared", "src", "token.ts"), `export const token = "ok";`)
	writeFile(t, filepath.Join(dir, "packages", "shared", "src", "orphan.ts"), `export const orphan = true;`)

	cfg := designPolicyTestConfig(dir)
	cfg.Targets[0].Entrypoints = []string{"src/main.ts"}
	cfg.Checks.DesignRules.Reachability = &codeguard.DesignReachabilityConfig{}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	finding := findFinding(t, report, "Design Patterns", "design.unreachable-module")
	if finding.Path != "packages/shared/src/orphan.ts" {
		t.Fatalf("unreachable finding path = %q, want packages/shared/src/orphan.ts", finding.Path)
	}
	for _, section := range report.Sections {
		for _, candidate := range section.Findings {
			if candidate.RuleID == "design.unreachable-module" && candidate.Path == "packages/shared/src/token.ts" {
				t.Fatal("workspace export target imported from the entrypoint was reported unreachable")
			}
		}
	}
}

func TestDesignReachabilityFollowsWorkspacePackageWildcardExports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.ts"), `import "@repo/shared/token";`)
	writeFile(t, filepath.Join(dir, "packages", "shared", "package.json"), "{\n  \"name\": \"@repo/shared\",\n  \"exports\": {\n    \"./*\": \"./src/*.ts\"\n  }\n}\n")
	writeFile(t, filepath.Join(dir, "packages", "shared", "src", "token.ts"), `export const token = "ok";`)
	writeFile(t, filepath.Join(dir, "packages", "shared", "src", "orphan.ts"), `export const orphan = true;`)

	cfg := designPolicyTestConfig(dir)
	cfg.Targets[0].Entrypoints = []string{"src/main.ts"}
	cfg.Checks.DesignRules.Reachability = &codeguard.DesignReachabilityConfig{}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	finding := findFinding(t, report, "Design Patterns", "design.unreachable-module")
	if finding.Path != "packages/shared/src/orphan.ts" {
		t.Fatalf("unreachable finding path = %q, want packages/shared/src/orphan.ts", finding.Path)
	}
	for _, section := range report.Sections {
		for _, candidate := range section.Findings {
			if candidate.RuleID == "design.unreachable-module" && candidate.Path == "packages/shared/src/token.ts" {
				t.Fatal("workspace wildcard export target imported from the entrypoint was reported unreachable")
			}
		}
	}
}

func TestDesignReachabilityFollowsCPPNamedModuleGraph(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.cpp"), "#include \"app.cppm\"\nint main() { return app_value(); }\n")
	writeFile(t, filepath.Join(dir, "src", "app.cppm"), "export module app;\nimport app.shared;\nexport int app_value();\n")
	writeFile(t, filepath.Join(dir, "src", "shared.cppm"), "export module app.shared;\nimport :detail;\nexport int shared_value();\n")
	writeFile(t, filepath.Join(dir, "src", "shared-detail.cppm"), "module app.shared:detail;\nexport int detail_value();\n")
	writeFile(t, filepath.Join(dir, "src", "orphan.cppm"), "export module app.orphan;\nexport int orphan_value();\n")

	cfg := designPolicyTestConfig(dir)
	cfg.Targets[0].Language = "cpp"
	cfg.Targets[0].Entrypoints = []string{"src/main.cpp"}
	cfg.Checks.DesignRules.Reachability = &codeguard.DesignReachabilityConfig{}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	finding := findFinding(t, report, "Design Patterns", "design.unreachable-module")
	if finding.Path != "src/orphan.cppm" {
		t.Fatalf("unreachable finding path = %q, want src/orphan.cppm", finding.Path)
	}
	for _, section := range report.Sections {
		for _, candidate := range section.Findings {
			if candidate.RuleID != "design.unreachable-module" {
				continue
			}
			switch candidate.Path {
			case "src/app.cppm", "src/shared.cppm", "src/shared-detail.cppm":
				t.Fatalf("reachable C++ module was reported unreachable: %+v", candidate)
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

func TestDesignStabilityReportsDependencyTowardVolatileCPPNamedModule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "one.cppm"), "export module app.one;\nimport app.stable;\nexport int one();\n")
	writeFile(t, filepath.Join(dir, "src", "two.cppm"), "export module app.two;\nimport app.stable;\nexport int two();\n")
	writeFile(t, filepath.Join(dir, "src", "three.cppm"), "export module app.three;\nimport app.stable;\nexport int three();\n")
	writeFile(t, filepath.Join(dir, "src", "stable.cppm"), "export module app.stable;\nimport app.volatile;\nexport int stable();\n")
	writeFile(t, filepath.Join(dir, "src", "volatile.cppm"), "export module app.volatile;\nimport app.leaf_one;\nimport app.leaf_two;\nimport app.leaf_three;\nimport app.leaf_four;\nexport int volatile_value();\n")
	writeFile(t, filepath.Join(dir, "src", "leaf_one.cppm"), "export module app.leaf_one;\nexport int leaf_one();\n")
	writeFile(t, filepath.Join(dir, "src", "leaf_two.cppm"), "export module app.leaf_two;\nexport int leaf_two();\n")
	writeFile(t, filepath.Join(dir, "src", "leaf_three.cppm"), "export module app.leaf_three;\nexport int leaf_three();\n")
	writeFile(t, filepath.Join(dir, "src", "leaf_four.cppm"), "export module app.leaf_four;\nexport int leaf_four();\n")

	cfg := designPolicyTestConfig(dir)
	cfg.Targets[0].Language = "cpp"
	cfg.Checks.DesignRules.Stability = &codeguard.DesignStabilityConfig{
		MinimumFanIn:        3,
		MaxInstabilityDelta: 0.30,
	}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	finding := findFinding(t, report, "Design Patterns", "design.stability-direction")
	if finding.Path != "src/stable.cppm" || finding.Line != 2 {
		t.Fatalf("stability finding location = %s:%d, want src/stable.cppm:2", finding.Path, finding.Line)
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
