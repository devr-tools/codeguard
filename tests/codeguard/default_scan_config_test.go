package codeguard_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func writeDefaultScanFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

// The SDK exposes the exact config the CLI falls back to without a config
// file, so an embedding tool gets the same repository-aware defaults.
func TestDefaultScanConfigDetectsTargetsAndSections(t *testing.T) {
	root := t.TempDir()
	writeDefaultScanFixture(t, root, map[string]string{
		"app/__main__.py":  "print(1)\n",
		"app/core.py":      "x = 1\n",
		"requirements.txt": "requests==2.32.3\n",
		"web/index.ts":     "export const x = 1;\n",
		"web/package.json": "{\"name\":\"web\",\"main\":\"index.ts\"}\n",
	})

	cfg, err := codeguard.DefaultScanConfig(root, "")
	if err != nil {
		t.Fatalf("DefaultScanConfig: %v", err)
	}
	languages := map[string]codeguard.TargetConfig{}
	for _, target := range cfg.Targets {
		if target.Path != root {
			t.Errorf("target %s path = %q, want the scan root %q", target.Name, target.Path, root)
		}
		languages[target.Language] = target
	}
	if len(languages) != 2 {
		t.Fatalf("targets = %+v, want python and typescript", cfg.Targets)
	}
	if got := languages["python"].Entrypoints; !slices.Equal(got, []string{"app/__main__.py"}) {
		t.Errorf("python entrypoints = %v", got)
	}
	if got := languages["typescript"].Entrypoints; !slices.Equal(got, []string{"web/index.ts"}) {
		t.Errorf("typescript entrypoints = %v", got)
	}
	if !cfg.Checks.SupplyChain {
		t.Error("supply_chain should be enabled when manifests exist")
	}
	if cfg.Checks.CI {
		t.Error("ci should stay off without .github/workflows")
	}
	if !cfg.Checks.Quality || !cfg.Checks.Security || !cfg.Checks.Design || !cfg.Checks.Prompts {
		t.Errorf("recommended sections should be on, got %+v", cfg.Checks)
	}
	if len(cfg.Checks.CIRules.RequiredReleaseFiles) != 0 || len(cfg.Checks.CIRules.RequiredAutomationPaths) != 0 {
		t.Errorf("defaults must not require codeguard's release files, got %+v", cfg.Checks.CIRules)
	}
}

func TestDefaultScanConfigRejectsUnknownProfile(t *testing.T) {
	if _, err := codeguard.DefaultScanConfig(t.TempDir(), "nope"); err == nil {
		t.Fatal("expected an unknown-profile error")
	}
}

// codeguard's own conventions are no longer defaults, so the self-scan config
// must state every one of them; otherwise `make codeguard-ci` silently stops
// checking them.
func TestSelfScanConfigStatesCodeguardConventions(t *testing.T) {
	cfg, err := codeguard.LoadConfigFile(filepath.Join("..", "..", ".codeguard", "codeguard.yaml"))
	if err != nil {
		t.Fatalf("load self-scan config: %v", err)
	}
	ci := cfg.Checks.CIRules
	for _, tc := range []struct {
		key  string
		have []string
		want []string
	}{
		{"ci_rules.required_release_files", ci.RequiredReleaseFiles, []string{".goreleaser.yaml", "CHANGELOG.md"}},
		{"ci_rules.required_automation_paths", ci.RequiredAutomationPaths, []string{"Makefile"}},
		{"ci_rules.required_workflow_files", ci.RequiredWorkflowFiles, []string{".github/workflows/ci.yml", ".github/workflows/release.yml"}},
		{"ci_rules.required_gates", ci.RequiredGates, []string{"test"}},
		{"ci_rules.allowed_test_paths", ci.AllowedTestPaths, []string{"tests/**"}},
	} {
		have := make(map[string]bool, len(tc.have))
		for _, value := range tc.have {
			have[value] = true
		}
		for _, want := range tc.want {
			if !have[want] {
				t.Errorf("self-scan %s = %v, missing %q", tc.key, tc.have, want)
			}
		}
	}
	if ci.RequireWorkflowDir == nil || !*ci.RequireWorkflowDir {
		t.Error("self-scan config must state require_workflow_dir: true")
	}
	markers := map[string]bool{}
	for _, rule := range ci.WorkflowContentRules {
		for _, marker := range rule.RequiredContains {
			markers[rule.Path+"|"+marker] = true
		}
	}
	for _, want := range []string{".github/workflows/ci.yml|make test", ".github/workflows/ci.yml|make codeguard-ci"} {
		if !markers[want] {
			t.Errorf("self-scan workflow_content_rules missing %s", want)
		}
	}
	design := cfg.Checks.DesignRules
	for name, value := range map[string]*bool{
		"require_cmd_through_internal_cli": design.RequireCmdThroughInternalCLI,
		"forbid_service_import_internal":   design.ForbidServiceImportInternal,
		"forbid_service_import_cmd":        design.ForbidServiceImportCmd,
	} {
		if value == nil || !*value {
			t.Errorf("self-scan design_rules.%s must be true", name)
		}
	}
	if !cfg.Checks.CI || !cfg.Checks.SupplyChain {
		t.Errorf("self-scan must run ci and supply_chain, got ci=%v supply_chain=%v", cfg.Checks.CI, cfg.Checks.SupplyChain)
	}
	if len(cfg.Targets) != 1 || cfg.Targets[0].Language != "go" || !slices.Contains(cfg.Targets[0].Entrypoints, "cmd/codeguard") {
		t.Errorf("self-scan target = %+v, want the go target with the cmd/codeguard entrypoint", cfg.Targets)
	}
}
