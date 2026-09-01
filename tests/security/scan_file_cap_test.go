package security_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// A repository-wide scan must not analyze dependency or generated build trees.
// Go vendor trees and TypeScript/CDK output can contain hundreds of thousands
// of source-shaped files; admitting them makes full-scan work and retained
// corpus memory proportional to installed dependencies instead of owned code.
func TestWalkFilesSkipsDependencyAndCDKOutputTrees(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"cmd/service/main.go":                                    "package main\n",
		"infrastructure/app.ts":                                  "export const app = {};\n",
		"vendor/example.com/dependency/dependency.go":            "package dependency\n",
		"infrastructure/node_modules/library/src/index.ts":       "export const dependency = {};\n",
		"infrastructure/cdk.out/asset.1234567890abcdef/index.js": "exports.generated = true;\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	got, err := runnersupport.WalkFiles(root, nil, func(string) bool { return true })
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	sort.Strings(got)
	want := []string{"cmd/service/main.go", "infrastructure/app.ts"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walked files = %v, want only owned source %v", got, want)
	}
}

func TestWalkFilesCanIncludeVendoredSourceWithoutIncludingInstalledOrGeneratedTrees(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"app/main.go": "package app\n",
		"vendor/example.com/dependency/dependency.go": "package dependency\n",
		"nested/vendor/local/patch.go":                "package local\n",
		"node_modules/library/index.js":               "exports.installed = true;\n",
		"nested/node_modules/library/index.js":        "exports.installed = true;\n",
		"infrastructure/cdk.out/asset/index.js":       "exports.generated = true;\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	got, err := runnersupport.WalkFilesWithOptions(root, []string{"nested/vendor/**"}, runnersupport.FileWalkOptions{
		ScanVendoredSource: true,
	}, func(string) bool { return true })
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	sort.Strings(got)
	want := []string{
		"app/main.go",
		"vendor/example.com/dependency/dependency.go",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walked files = %v, want owned and vendored source %v", got, want)
	}
}

func TestWalkFilesAppliesDependencyPolicyToLogicalTargetRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "dependency.go"), []byte("package dependency\n"), 0o600); err != nil {
		t.Fatalf("write dependency source: %v", err)
	}

	for _, tc := range []struct {
		name               string
		logicalPath        string
		scanVendoredSource bool
		wantFiles          []string
	}{
		{name: "vendor-default", logicalPath: "vendor/example.com/dependency"},
		{name: "vendor-opt-in", logicalPath: "vendor/example.com/dependency", scanVendoredSource: true, wantFiles: []string{"dependency.go"}},
		{name: "nested-node-modules", logicalPath: "web/node_modules/library", scanVendoredSource: true},
		{name: "nested-cdk-output", logicalPath: "infrastructure/cdk.out/asset", scanVendoredSource: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := runnersupport.WalkFilesWithOptions(root, nil, runnersupport.FileWalkOptions{
				LogicalPath:        tc.logicalPath,
				ScanVendoredSource: tc.scanVendoredSource,
			}, func(string) bool { return true })
			if err != nil {
				t.Fatalf("walk: %v", err)
			}
			if !reflect.DeepEqual(got, tc.wantFiles) {
				t.Fatalf("walked files = %v, want %v", got, tc.wantFiles)
			}
		})
	}
}

func TestFullScanIncludesVendoredSourceWhenConfigured(t *testing.T) {
	root := t.TempDir()
	vendoredPath := filepath.Join(root, "vendor", "example.com", "dependency", "dependency.go")
	if err := os.MkdirAll(filepath.Dir(vendoredPath), 0o755); err != nil {
		t.Fatalf("mkdir vendor: %v", err)
	}
	if err := os.WriteFile(vendoredPath, []byte(strings.Repeat("// vendored source\n", 8)), 0o600); err != nil {
		t.Fatalf("write vendored source: %v", err)
	}

	cfg := codeguard.ExampleConfig()
	cfg.Name = "vendored-source-opt-in"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: root, Language: "go"}}
	cfg.ScanVendoredSource = true
	cfg.Checks.Quality = true
	cfg.Checks.QualityRules.MaxFileLines = 2
	cfg.Checks.QualityRules.MaxFunctionLines = 100
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	cacheEnabled := true
	cfg.Cache.Enabled = &cacheEnabled
	cfg.Cache.Path = filepath.Join(root, ".codeguard", "vendored-source-cache.json")

	cfg.ScanVendoredSource = false
	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull})
	if err != nil {
		t.Fatalf("default full scan: %v", err)
	}
	if reportHasFindingPath(report, "vendor/example.com/dependency/dependency.go") {
		t.Fatal("default full scan included vendored source")
	}

	cfg.ScanVendoredSource = true
	report, err = codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull})
	if err != nil {
		t.Fatalf("opt-in full scan: %v", err)
	}
	if !reportHasFindingPath(report, "vendor/example.com/dependency/dependency.go") {
		t.Fatal("opt-in full scan did not report the oversized vendored source after a default cached scan")
	}

	cfg.ScanVendoredSource = false
	report, err = codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull})
	if err != nil {
		t.Fatalf("second default full scan: %v", err)
	}
	if reportHasFindingPath(report, "vendor/example.com/dependency/dependency.go") {
		t.Fatal("default full scan replayed a cached vendored-source finding")
	}
}

func TestConfigFileTargetRootHonorsVendoredSourcePolicy(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "vendor"), 0o755); err != nil {
		t.Fatalf("mkdir vendor: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "vendor", "dependency.go"), []byte(strings.Repeat("// vendored source\n", 8)), 0o600); err != nil {
		t.Fatalf("write vendored source: %v", err)
	}

	for _, tc := range []struct {
		name    string
		enabled bool
	}{
		{name: "default"},
		{name: "opt-in", enabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(root, "codeguard-"+tc.name+".json")
			configJSON := `{
  "name": "vendored-target-root",
  "targets": [{"name": "vendor", "path": "vendor", "language": "go"}],
  "scan_vendored_source": ` + fmt.Sprintf("%t", tc.enabled) + `,
  "checks": {"quality": true, "quality_rules": {"max_file_lines": 2}, "security_rules": {"govulncheck_mode": "off"}},
  "output": {"format": "json"}
}`
			if err := os.WriteFile(configPath, []byte(configJSON), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			cfg, err := codeguard.LoadConfigFile(configPath)
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull})
			if err != nil {
				t.Fatalf("full scan: %v", err)
			}
			if got := reportHasFindingPath(report, "dependency.go"); got != tc.enabled {
				t.Fatalf("vendored finding present = %t, want %t", got, tc.enabled)
			}
		})
	}
}

func TestRelativeConfigTargetPreservesTargetRelativeExcludes(t *testing.T) {
	root := t.TempDir()
	for rel, content := range map[string]string{
		"services/api/main.go":           strings.Repeat("// owned source\n", 8),
		"services/api/dist/bundle.go":    strings.Repeat("// distribution output\n", 8),
		"services/api/generated/code.go": strings.Repeat("// configured exclusion\n", 8),
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	configPath := filepath.Join(root, "codeguard.json")
	configJSON := `{
  "name": "relative-target-excludes",
  "targets": [{"name": "api", "path": "services/api", "language": "go"}],
  "exclude": ["generated/**"],
  "checks": {"quality": true, "quality_rules": {"max_file_lines": 2}, "security_rules": {"govulncheck_mode": "off"}},
  "output": {"format": "json"}
}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := codeguard.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull})
	if err != nil {
		t.Fatalf("full scan: %v", err)
	}
	if !reportHasFindingPath(report, "main.go") {
		t.Fatal("owned source was not scanned")
	}
	for _, excluded := range []string{"dist/bundle.go", "generated/code.go"} {
		if reportHasFindingPath(report, excluded) {
			t.Fatalf("target-relative exclusion did not exclude %s", excluded)
		}
	}
}

func TestAbsoluteTargetRootHonorsDependencyPolicyWithoutInspectingAncestors(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"vendor/dependency.go",
		"node_modules/dependency.go",
		"cdk.out/dependency.go",
		"unrelated/vendor/projects/app/main.go",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(strings.Repeat("// source\n", 8)), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	for _, tc := range []struct {
		name               string
		target             string
		scanVendoredSource bool
		wantFinding        bool
	}{
		{name: "vendor-default", target: "vendor"},
		{name: "vendor-opt-in", target: "vendor", scanVendoredSource: true, wantFinding: true},
		{name: "node-modules-hard-exclusion", target: "node_modules", scanVendoredSource: true},
		{name: "cdk-output-hard-exclusion", target: "cdk.out", scanVendoredSource: true},
		{name: "unrelated-vendor-ancestor", target: "unrelated/vendor/projects/app", wantFinding: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := codeguard.ExampleConfig()
			cfg.Name = "absolute-target-root-" + tc.name
			cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: filepath.Join(root, filepath.FromSlash(tc.target)), Language: "go"}}
			cfg.ScanVendoredSource = tc.scanVendoredSource
			cfg.Checks.Quality = true
			cfg.Checks.QualityRules.MaxFileLines = 2
			cfg.Checks.SecurityRules.GovulncheckMode = "off"

			report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull})
			if err != nil {
				t.Fatalf("full scan: %v", err)
			}
			findingPath := "dependency.go"
			if tc.name == "unrelated-vendor-ancestor" {
				findingPath = "main.go"
			}
			if got := reportHasFindingPath(report, findingPath); got != tc.wantFinding {
				t.Fatalf("finding present = %t, want %t", got, tc.wantFinding)
			}
		})
	}
}

func TestFullScanFolderCannotBypassVendoredSourceDefault(t *testing.T) {
	root := t.TempDir()
	vendorRoot := filepath.Join(root, "nested", "vendor")
	vendoredPath := filepath.Join(vendorRoot, "dependency", "dependency.go")
	if err := os.MkdirAll(filepath.Dir(vendoredPath), 0o755); err != nil {
		t.Fatalf("mkdir vendor: %v", err)
	}
	if err := os.WriteFile(vendoredPath, []byte(strings.Repeat("// vendored source\n", 8)), 0o600); err != nil {
		t.Fatalf("write vendored source: %v", err)
	}

	cfg := codeguard.ExampleConfig()
	cfg.Name = "vendored-source-folder-default"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: root, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.QualityRules.MaxFileLines = 2
	cfg.Checks.SecurityRules.GovulncheckMode = "off"

	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:       codeguard.ScanModeFull,
		TargetPath: vendorRoot,
	})
	if err != nil {
		t.Fatalf("full folder scan: %v", err)
	}
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.Path == "dependency/dependency.go" {
				t.Fatal("folder scan bypassed the default vendored-source exclusion")
			}
		}
	}
}

func reportHasFindingPath(report codeguard.Report, path string) bool {
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.Path == path {
				return true
			}
		}
	}
	return false
}

// An untrusted repository must not be able to exhaust scan memory with an
// oversized file: files above the scan size cap are skipped by the walk so no
// section ever reads them into memory. Normal-sized files are still listed.
func TestWalkFilesSkipsOversizedFiles(t *testing.T) {
	root := t.TempDir()

	small := filepath.Join(root, "small.go")
	if err := os.WriteFile(small, []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write small file: %v", err)
	}

	// A sparse file just over the 32 MiB cap; truncate does not allocate blocks.
	big := filepath.Join(root, "big.go")
	f, err := os.Create(big)
	if err != nil {
		t.Fatalf("create big file: %v", err)
	}
	if err = f.Truncate((32 << 20) + 1); err != nil {
		t.Fatalf("truncate big file: %v", err)
	}
	if err = f.Close(); err != nil {
		t.Fatalf("close big file: %v", err)
	}

	files, err := runnersupport.WalkFiles(root, nil, func(string) bool { return true })
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	got := map[string]bool{}
	for _, rel := range files {
		got[rel] = true
	}
	if !got["small.go"] {
		t.Errorf("expected small.go to be listed, got %v", files)
	}
	if got["big.go"] {
		t.Errorf("oversized big.go must be skipped, got %v", files)
	}
}
