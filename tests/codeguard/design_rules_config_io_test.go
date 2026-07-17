package codeguard_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestLoadFileExplicitDesignRulesYAMLInlinePrecedence(t *testing.T) {
	root := t.TempDir()
	writeConfigTestFile(t, filepath.Join(root, ".codeguard", "policy.yml"), `
max_decls_per_file: 99
detect_import_cycles: true
forbidden_package_names: [legacy]
`)
	configPath := filepath.Join(root, "codeguard.yml")
	writeConfigTestFile(t, configPath, `
name: external-policy
targets:
  - name: repo
    path: .
    language: go
checks:
  design_rules_file: .codeguard/policy.yml
  design_rules:
    max_decls_per_file: 0
    detect_import_cycles: false
    forbidden_package_names: []
output:
  format: text
`)

	cfg, err := codeguard.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if cfg.Checks.DesignRules.MaxDeclsPerFile != 0 {
		t.Fatalf("max_decls_per_file = %d, want explicit inline zero", cfg.Checks.DesignRules.MaxDeclsPerFile)
	}
	if cfg.Checks.DesignRules.DetectImportCycles == nil || *cfg.Checks.DesignRules.DetectImportCycles {
		t.Fatal("detect_import_cycles did not preserve explicit inline false")
	}
	if cfg.Checks.DesignRules.ForbiddenPackageNames == nil || len(cfg.Checks.DesignRules.ForbiddenPackageNames) != 0 {
		t.Fatalf("forbidden_package_names = %#v, want explicit empty list", cfg.Checks.DesignRules.ForbiddenPackageNames)
	}
}

func TestLoadFileExternalDesignRulesPreserveExplicitZeroAndEmptyValues(t *testing.T) {
	root := t.TempDir()
	writeConfigTestFile(t, filepath.Join(root, ".codeguard", "design_rules.yml"), `
max_decls_per_file: 0
forbidden_package_names: []
`)
	configPath := filepath.Join(root, "codeguard.yml")
	writeConfigTestFile(t, configPath, `
name: external-zero-values
profile: strict
targets:
  - name: repo
    path: .
    language: go
checks: {}
output:
  format: text
`)

	cfg, err := codeguard.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if cfg.Checks.DesignRules.MaxDeclsPerFile != 0 {
		t.Fatalf("max_decls_per_file = %d, want explicit external zero", cfg.Checks.DesignRules.MaxDeclsPerFile)
	}
	if cfg.Checks.DesignRules.ForbiddenPackageNames == nil || len(cfg.Checks.DesignRules.ForbiddenPackageNames) != 0 {
		t.Fatalf("forbidden_package_names = %#v, want explicit external empty list", cfg.Checks.DesignRules.ForbiddenPackageNames)
	}
}

func TestLoadFileExplicitDesignRulesJSONMainRelativeToConfigDirectory(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".codeguard")
	writeConfigTestFile(t, filepath.Join(configDir, "team-policy.yml"), `
max_methods_per_type: 31
detect_god_modules: false
`)
	configPath := filepath.Join(configDir, "codeguard.json")
	writeConfigTestFile(t, configPath, `{
  "name": "json-external-policy",
  "targets": [{"name": "repo", "path": "..", "language": "go"}],
  "checks": {
    "design_rules_file": "team-policy.yml",
    "design_rules": {"max_methods_per_type": 7}
  },
  "output": {"format": "json"}
}`)

	cfg, err := codeguard.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if got := cfg.Checks.DesignRules.MaxMethodsPerType; got != 7 {
		t.Fatalf("max_methods_per_type = %d, want inline value 7", got)
	}
	if cfg.Checks.DesignRules.DetectGodModules == nil || *cfg.Checks.DesignRules.DetectGodModules {
		t.Fatal("external detect_god_modules=false was not loaded")
	}
}

func TestLoadFileDiscoversDesignRules(t *testing.T) {
	tests := []struct {
		name       string
		configPath func(string) string
		policyPath func(string) string
	}{
		{
			name:       "project dot-codeguard directory",
			configPath: func(root string) string { return filepath.Join(root, "codeguard.yml") },
			policyPath: func(root string) string { return filepath.Join(root, ".codeguard", "design_rules.yml") },
		},
		{
			name:       "beside config already in dot-codeguard",
			configPath: func(root string) string { return filepath.Join(root, ".codeguard", "codeguard.yml") },
			policyPath: func(root string) string { return filepath.Join(root, ".codeguard", "design_rules.yaml") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeConfigTestFile(t, tt.policyPath(root), "max_interface_methods: 23\n")
			writeConfigTestFile(t, tt.configPath(root), `
name: discovered-policy
targets:
  - name: repo
    path: .
    language: go
checks: {}
output:
  format: text
`)

			cfg, err := codeguard.LoadConfigFile(tt.configPath(root))
			if err != nil {
				t.Fatalf("LoadFile: %v", err)
			}
			if got := cfg.Checks.DesignRules.MaxInterfaceMethods; got != 23 {
				t.Fatalf("max_interface_methods = %d, want discovered value 23", got)
			}
		})
	}
}

func TestLoadFileDesignRulesErrors(t *testing.T) {
	t.Run("invalid external document", func(t *testing.T) {
		root := t.TempDir()
		writeConfigTestFile(t, filepath.Join(root, ".codeguard", "design_rules.yml"), "layers: [\n")
		configPath := filepath.Join(root, "codeguard.yml")
		writeConfigTestFile(t, configPath, minimalConfigYAML(""))

		_, err := codeguard.LoadConfigFile(configPath)
		if err == nil || !strings.Contains(err.Error(), "parse design rules file") {
			t.Fatalf("error = %v, want invalid design rules error", err)
		}
	})

	t.Run("missing explicit file", func(t *testing.T) {
		root := t.TempDir()
		configPath := filepath.Join(root, "codeguard.yml")
		writeConfigTestFile(t, configPath, minimalConfigYAML("missing.yml"))

		_, err := codeguard.LoadConfigFile(configPath)
		if err == nil || !strings.Contains(err.Error(), "design_rules_file") {
			t.Fatalf("error = %v, want missing explicit file error", err)
		}
	})

	t.Run("path traversal", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "repo")
		writeConfigTestFile(t, filepath.Join(parent, "outside.yml"), "max_decls_per_file: 2\n")
		configPath := filepath.Join(root, ".codeguard", "codeguard.yml")
		writeConfigTestFile(t, configPath, minimalConfigYAML("../../outside.yml"))

		_, err := codeguard.LoadConfigFile(configPath)
		if err == nil || !strings.Contains(err.Error(), "escapes") {
			t.Fatalf("error = %v, want traversal rejection", err)
		}
	})

	t.Run("symlink escape", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "repo")
		outside := filepath.Join(parent, "outside.yml")
		writeConfigTestFile(t, outside, "max_decls_per_file: 2\n")
		link := filepath.Join(root, ".codeguard", "linked.yml")
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		configPath := filepath.Join(root, ".codeguard", "codeguard.yml")
		writeConfigTestFile(t, configPath, minimalConfigYAML("linked.yml"))

		_, err := codeguard.LoadConfigFile(configPath)
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("error = %v, want symlink escape rejection", err)
		}
	})
}

func minimalConfigYAML(designRulesFile string) string {
	fileSetting := ""
	if designRulesFile != "" {
		fileSetting = "  design_rules_file: " + designRulesFile + "\n"
	}
	return `name: policy-errors
targets:
  - name: repo
    path: .
    language: go
checks:
` + fileSetting + `output:
  format: text
`
}

func writeConfigTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
