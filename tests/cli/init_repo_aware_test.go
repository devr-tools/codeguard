package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// codeguardSpecificRules only fire because of codeguard's own repository
// conventions (its workflow files and content markers, release files,
// Makefile, tests/ layout and cmd/ -> internal/cli layering). A config written
// by `codeguard init` for some other repository must never produce them.
var codeguardSpecificRules = map[string]bool{
	"ci.required-file":                true,
	"ci.workflow-content":             true,
	"ci.test-file-location":           true,
	"ci.missing-required-gate":        true,
	"ci.required-workflow-dir":        true,
	"design.service-import-internal":  true,
	"design.service-import-cmd":       true,
	"design.cmd-through-internal-cli": true,
}

func writeFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		writeScanTestFile(t, filepath.Join(root, filepath.FromSlash(rel)), content)
	}
}

// goOnlyFixture is a clean Go repository with a cmd/ binary, co-located tests,
// a pkg/ package importing internal/, and a workflow that tests via make — all
// ordinary, and all failures under codeguard's own conventions.
func goOnlyFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		"go.mod":                     "module example.com/goonly\n\ngo 1.23.0\n",
		"cmd/tool/main.go":           "package main\n\nimport \"example.com/goonly/pkg/api\"\n\nfunc main() {\n\tprintln(api.Sum(1, 2))\n}\n",
		"internal/calc/calc.go":      "package calc\n\n// Add returns the sum of a and b.\nfunc Add(a int, b int) int { return a + b }\n",
		"internal/calc/calc_test.go": "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"bad sum\")\n\t}\n}\n",
		"pkg/api/api.go":             "package api\n\nimport \"example.com/goonly/internal/calc\"\n\n// Sum adds two numbers.\nfunc Sum(a int, b int) int { return calc.Add(a, b) }\n",
		".github/workflows/ci.yml":   "name: ci\non: [push]\npermissions:\n  contents: read\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@8f3c2b1a4d5e6f7890abc1234567890abc123456\n      - run: make check\n",
		"README.md":                  "# goonly\n\nRun `make check`.\n",
		"AGENTS.md":                  "# Agent Notes\n\n## Build & test\n- `make check` runs the suite.\n",
	})
	return root
}

// pythonTSFixture is a Python package with a console script beside a
// TypeScript package with a bin, and no CI workflows.
func pythonTSFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		"pyproject.toml":     "[project]\nname = \"pyapp\"\nversion = \"0.1.0\"\ndependencies = [\"requests==2.32.3\"]\n\n[project.scripts]\npyapp = \"pyapp.cli:main\"\n",
		"pyapp/__init__.py":  "",
		"pyapp/cli.py":       "from pyapp.core import greet\n\n\ndef main() -> int:\n    print(greet(\"world\"))\n    return 0\n",
		"pyapp/core.py":      "def greet(name: str) -> str:\n    return f\"hello {name}\"\n",
		"web/package.json":   "{\"name\":\"web\",\"version\":\"1.0.0\",\"main\":\"dist/index.js\",\"bin\":{\"webtool\":\"bin/webtool.js\"},\"dependencies\":{\"lodash\":\"*\"}}\n",
		"web/bin/webtool.js": "#!/usr/bin/env node\nconsole.log(\"hi\");\n",
		"web/src/index.ts":   "export function greet(name: string): string {\n  return `hi ${name}`;\n}\n",
		"web/src/format.ts":  "export function shout(value: string): string {\n  return value.toUpperCase();\n}\n",
		"web/dist/index.js":  "module.exports = {};\n",
		"README.md":          "# pyapp\n",
	})
	return root
}

// monorepoFixture has a root package.json workspace, a TS package, a Python
// service with a __main__.py, and dependency/tool trees that must not count.
func monorepoFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"package.json":                      "{\"name\":\"mono\",\"private\":true,\"workspaces\":[\"packages/*\"]}\n",
		"packages/ui/package.json":          "{\"name\":\"ui\",\"main\":\"src/index.ts\"}\n",
		"packages/ui/src/index.ts":          "export const ui = 1;\n",
		"packages/ui/src/button.tsx":        "export const Button = () => null;\n",
		"services/api/pyproject.toml":       "[project]\nname = \"api\"\nversion = \"0.1.0\"\n",
		"services/api/api/__init__.py":      "",
		"services/api/api/__main__.py":      "print(\"api\")\n",
		"services/api/api/routes.py":        "def routes() -> list:\n    return []\n",
		".github/workflows/ci.yml":          "name: ci\non: [push]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: pytest\n",
		"services/api/testdata/sample.go":   "package sample\n",
		".claude/worktrees/copy/main.go":    "package main\n",
		"packages/ui/dist/bundle.js":        "var a=1;\n",
		"services/api/.venv/lib/site.py":    "x = 1\n",
		"packages/ui/node_modules/dep/a.js": "module.exports = 1;\n",
	}
	// Enough Ruby inside node_modules to earn a target if the walk counted it.
	for i := 0; i < 40; i++ {
		files[fmt.Sprintf("node_modules/gem/lib/file%d.rb", i)] = "puts 1\n"
	}
	writeFixture(t, root, files)
	return root
}

func runInit(t *testing.T, args ...string) (string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := cli.Run(append([]string{"init"}, args...), strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("init %v exit = %d, stderr = %s", args, code, stderr.String())
	}
	return stdout.String(), stderr.String()
}

func initFixture(t *testing.T, root string, extra ...string) (codeguard.Config, string) {
	t.Helper()
	configPath := filepath.Join(root, "codeguard.yaml")
	stdout, _ := runInit(t, append([]string{"-path", root, "-output", configPath}, extra...)...)
	cfg, err := codeguard.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	return cfg, stdout
}

type targetSummary struct {
	Language    string
	Entrypoints []string
}

func summarizeTargets(cfg codeguard.Config) map[string]targetSummary {
	out := map[string]targetSummary{}
	for _, target := range cfg.Targets {
		out[target.Name] = targetSummary{Language: target.Language, Entrypoints: target.Entrypoints}
	}
	return out
}

func TestInitGoOnlyRepository(t *testing.T) {
	root := goOnlyFixture(t)
	cfg, stdout := initFixture(t, root)

	want := map[string]targetSummary{"go": {Language: "go", Entrypoints: []string{"cmd/tool"}}}
	if got := summarizeTargets(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("targets = %+v, want %+v", got, want)
	}
	if !cfg.Checks.CI {
		t.Error("ci should be enabled: .github/workflows exists")
	}
	if !cfg.Checks.SupplyChain {
		t.Error("supply_chain should be enabled: go.mod is a manifest")
	}
	if !strings.Contains(stdout, "detected languages: go") || !strings.Contains(stdout, "wrote ") {
		t.Errorf("init output should summarize detection, got:\n%s", stdout)
	}
}

func TestInitPythonAndTypeScriptRepository(t *testing.T) {
	root := pythonTSFixture(t)
	cfg, _ := initFixture(t, root)

	want := map[string]targetSummary{
		"python":     {Language: "python", Entrypoints: []string{"pyapp/cli.py"}},
		"typescript": {Language: "typescript", Entrypoints: []string{"web/bin/webtool.js"}},
	}
	if got := summarizeTargets(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("targets = %+v, want %+v (dist/ main is build output, not an entrypoint)", got, want)
	}
	if cfg.Checks.CI {
		t.Error("ci should be disabled: there is no .github/workflows directory")
	}
	if !cfg.Checks.SupplyChain {
		t.Error("supply_chain should be enabled: pyproject.toml and package.json exist")
	}

	data, err := os.ReadFile(filepath.Join(root, "codeguard.yaml"))
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	text := string(data)
	if lines := strings.Count(text, "\n"); lines > 60 {
		t.Errorf("generated config has %d lines; init must write a small config, not the full dump", lines)
	}
	for _, leaked := range []string{"cmd/codeguard", "language: go", "goreleaser", "go test ./...", "Makefile", "critical_path_patterns"} {
		if strings.Contains(text, leaked) {
			t.Errorf("generated config contains codeguard's own setting %q:\n%s", leaked, text)
		}
	}
	if !strings.Contains(text, "docs/checks.md") {
		t.Errorf("generated config should point at docs/checks.md for everything else:\n%s", text)
	}
}

func TestInitMonorepoIgnoresVendoredAndGeneratedTrees(t *testing.T) {
	root := monorepoFixture(t)
	cfg, _ := initFixture(t, root)

	got := summarizeTargets(cfg)
	names := make([]string, 0, len(got))
	for name := range got {
		names = append(names, name)
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"python", "typescript"}) {
		t.Fatalf("targets = %v, want [python typescript]; node_modules, .venv, dist, testdata and hidden dirs must not count", names)
	}
	if want := []string{"services/api/api/__main__.py"}; !reflect.DeepEqual(got["python"].Entrypoints, want) {
		t.Errorf("python entrypoints = %v, want %v", got["python"].Entrypoints, want)
	}
	if want := []string{"packages/ui/src/index.ts"}; !reflect.DeepEqual(got["typescript"].Entrypoints, want) {
		t.Errorf("typescript entrypoints = %v, want %v", got["typescript"].Entrypoints, want)
	}
	if !cfg.Checks.CI || !cfg.Checks.SupplyChain {
		t.Errorf("ci and supply_chain should both be enabled, got ci=%v supply_chain=%v", cfg.Checks.CI, cfg.Checks.SupplyChain)
	}
}

func TestInitFullWritesEveryKeyForTheDetectedRepository(t *testing.T) {
	root := pythonTSFixture(t)
	initFixture(t, root, "-full")

	data, err := os.ReadFile(filepath.Join(root, "codeguard.yaml"))
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	text := string(data)
	if strings.Count(text, "\n") < 200 {
		t.Errorf("-full should write every setting, got %d lines", strings.Count(text, "\n"))
	}
	if !strings.Contains(text, "language: python") || strings.Contains(text, "cmd/codeguard") {
		t.Errorf("-full should expand the detected config, not codeguard's own starter")
	}
}

func TestInitIntoDotCodeguardPointsTargetsAtRepositoryRoot(t *testing.T) {
	root := goOnlyFixture(t)
	configPath := filepath.Join(root, ".codeguard", "codeguard.yaml")
	runInit(t, "-path", root, "-output", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	if !strings.Contains(string(data), "path: ..") {
		t.Fatalf("a config in .codeguard/ must name the repository root as \"..\":\n%s", data)
	}
	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"validate", "-config", configPath}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("validate exit = %d, stderr = %s", code, stderr.String())
	}
}

type scanJSONReport struct {
	Sections []struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Findings []struct {
			RuleID string `json:"rule_id"`
			Level  string `json:"level"`
			Path   string `json:"path"`
		} `json:"findings"`
	} `json:"sections"`
}

func decodeScanReport(t *testing.T, stdout string, stderr string) scanJSONReport {
	t.Helper()
	var report scanJSONReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode report: %v\nstdout = %s\nstderr = %s", err, stdout, stderr)
	}
	if len(report.Sections) == 0 {
		t.Fatalf("scan reported no sections; stderr = %s", stderr)
	}
	return report
}

// The first scan of a clean repository with the config `init` wrote must not
// fail on codeguard's own conventions. This is the original defect: init wrote
// codeguard's settings and the first scan failed on .goreleaser.yaml, Makefile
// and the `go test ./...` workflow marker.
func TestFirstScanAfterInitDoesNotFailOnCodeguardConventions(t *testing.T) {
	root := goOnlyFixture(t)
	initFixture(t, root)

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"scan", "-config", filepath.Join(root, "codeguard.yaml"), "-format", "json"}, strings.NewReader(""), &stdout, &stderr)
	report := decodeScanReport(t, stdout.String(), stderr.String())
	for _, section := range report.Sections {
		if section.ID == "ci" && section.Status == "fail" {
			t.Errorf("ci section failed on a clean repository: %+v", section.Findings)
		}
		for _, finding := range section.Findings {
			if codeguardSpecificRules[finding.RuleID] {
				t.Errorf("first scan reported codeguard-specific rule %s at %s", finding.RuleID, finding.Path)
			}
		}
	}
	if code != 0 {
		t.Errorf("first scan of a clean repository exit = %d, want 0; stderr = %s\nstdout = %s", code, stderr.String(), stdout.String())
	}
}

// Multi-language repositories get one target per language over the same root;
// a language-agnostic finding must still be reported once, not once per target.
func TestMultiTargetScanReportsSharedFindingsOnce(t *testing.T) {
	root := pythonTSFixture(t)
	initFixture(t, root)

	var stdout, stderr bytes.Buffer
	cli.Run([]string{"scan", "-config", filepath.Join(root, "codeguard.yaml"), "-format", "json"}, strings.NewReader(""), &stdout, &stderr)
	report := decodeScanReport(t, stdout.String(), stderr.String())
	lodash := 0
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == "supply_chain.unpinned-dependency" && finding.Path == "web/package.json" {
				lodash++
			}
		}
	}
	if lodash != 1 {
		t.Fatalf("\"lodash\": \"*\" reported %d times, want exactly once", lodash)
	}
}

func chdirTemp(t *testing.T, dir string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
}

func ruleSet(report scanJSONReport) map[string]bool {
	rules := map[string]bool{}
	for _, section := range report.Sections {
		rules["section:"+section.ID] = true
		for _, finding := range section.Findings {
			rules[finding.RuleID+"@"+finding.Path] = true
		}
	}
	return rules
}

// Plain `codeguard scan` with no config file falls back to the same built-in
// defaults as `scan -folder .` (it used to exit with "config file not found"),
// and says so in one line on stderr.
func TestConfiglessScanFallsBackToDefaultsWithInitHint(t *testing.T) {
	root := pythonTSFixture(t)
	chdirTemp(t, root)

	var stdout, stderr bytes.Buffer
	cli.Run([]string{"scan", "-format", "json"}, strings.NewReader(""), &stdout, &stderr)
	if strings.Contains(stderr.String(), "config file not found") {
		t.Fatalf("config-less scan still errors: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "codeguard init") {
		t.Errorf("config-less scan should print a hint to run `codeguard init`, stderr = %s", stderr.String())
	}
	plain := decodeScanReport(t, stdout.String(), stderr.String())
	if !ruleSet(plain)["section:supply_chain"] {
		t.Errorf("config-less scan should enable supply_chain when manifests exist; sections = %v", ruleSet(plain))
	}

	stdout.Reset()
	stderr.Reset()
	cli.Run([]string{"scan", "-folder", ".", "-format", "json"}, strings.NewReader(""), &stdout, &stderr)
	folder := decodeScanReport(t, stdout.String(), stderr.String())
	if !reflect.DeepEqual(ruleSet(plain), ruleSet(folder)) {
		t.Errorf("`scan` and `scan -folder .` disagree without a config:\nscan:    %v\n-folder: %v", ruleSet(plain), ruleSet(folder))
	}
}

// A config path the operator named explicitly must still exist.
func TestScanWithExplicitMissingConfigStillErrors(t *testing.T) {
	chdirTemp(t, t.TempDir())
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"scan", "-config", "missing.yaml"}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "config file not found") {
		t.Fatalf("exit = %d, stderr = %q; an explicit missing -config must fail", code, stderr.String())
	}
}
