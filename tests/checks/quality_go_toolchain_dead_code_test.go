package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

const goToolchainDeadCodeRuleID = "quality.dead-code.toolchain"

func TestGoToolchainDeadCodeIsOptIn(t *testing.T) {
	dir := t.TempDir()
	writeGoToolchainModule(t, dir)
	writeFile(t, filepath.Join(dir, "cmd", "app", "main.go"), `package main

import "example.com/toolchaindeadcode/internal/service"

func main() {
	service.Run()
}
`)
	writeFile(t, filepath.Join(dir, "internal", "service", "service.go"), `package service

func Run() {}

func unusedPrivateHelper() {}
`)

	report := runGoToolchainDeadCodeScan(t, dir, false)

	assertFindingRuleAbsent(t, report, "Code Quality", goToolchainDeadCodeRuleID)
}

func TestGoToolchainDeadCodeReportsUnusedPrivateGoFunctions(t *testing.T) {
	dir := t.TempDir()
	writeGoToolchainModule(t, dir)
	writeFile(t, filepath.Join(dir, "cmd", "app", "main.go"), `package main

import "example.com/toolchaindeadcode/internal/service"

func main() {
	service.Run()
}
`)
	writeFile(t, filepath.Join(dir, "internal", "service", "service.go"), `package service

func Run() int {
	return usedHelper()
}

func usedHelper() int { return 1 }

func unusedPrivateHelper() int { return 2 }
`)

	report := runGoToolchainDeadCodeScan(t, dir, true)

	assertToolchainDeadCodeFindingPath(t, report, "internal/service/service.go")
}

func TestGoToolchainDeadCodeReportsPackagesUnreachableFromGoEntrypoints(t *testing.T) {
	dir := t.TempDir()
	writeGoToolchainModule(t, dir)
	writeFile(t, filepath.Join(dir, "cmd", "app", "main.go"), `package main

import "example.com/toolchaindeadcode/internal/live"

func main() {
	live.Run()
}
`)
	writeFile(t, filepath.Join(dir, "internal", "live", "live.go"), `package live

func Run() {}
`)
	writeFile(t, filepath.Join(dir, "internal", "orphan", "orphan.go"), `package orphan

func Run() {}
`)

	report := runGoToolchainDeadCodeScan(t, dir, true)

	assertToolchainDeadCodeFindingPath(t, report, "internal/orphan/orphan.go")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "internal/live/live.go")
}

func TestGoToolchainDeadCodeAvoidsKnownEntrypointAndAPIFalsePositives(t *testing.T) {
	dir := t.TempDir()
	writeGoToolchainModule(t, dir)
	writeFile(t, filepath.Join(dir, "cmd", "app", "main.go"), `package main

import "example.com/toolchaindeadcode/internal/service"

func main() {
	service.Run()
}
`)
	writeFile(t, filepath.Join(dir, "internal", "service", "service.go"), `package service

import "reflect"

type Runner interface {
	run() string
}

type serviceRunner struct{}

func init() {
	register("startup", exportedByDirective)
}

func Run() string {
	var runner Runner = serviceRunner{}
	reflect.ValueOf(reflectedHook).Pointer()
	hookRegistry["stringRegisteredHook"]()
	return runner.run()
}

func ExportedAPI() string { return "public" }

func (serviceRunner) run() string { return "interface implementation" }

var hookRegistry = map[string]func(){
	"stringRegisteredHook": stringRegisteredHook,
}

func reflectedHook() {}

func stringRegisteredHook() {}

func exportedByDirective() {}

func register(name string, hook func()) {}
`)
	writeFile(t, filepath.Join(dir, "internal", "service", "service_test.go"), `package service

import "testing"

func TestRun(t *testing.T) {
	if Run() == "" {
		t.Fatal("empty")
	}
}

func helperOnlyForTests() {}
`)

	report := runGoToolchainDeadCodeScan(t, dir, true)

	assertFindingRuleAbsent(t, report, "Code Quality", goToolchainDeadCodeRuleID)
}

func TestGoToolchainDeadCodeAvoidsGoLinknameDirectiveFalsePositive(t *testing.T) {
	dir := t.TempDir()
	writeGoToolchainModule(t, dir)
	writeFile(t, filepath.Join(dir, "cmd", "app", "main.go"), `package main

import _ "example.com/toolchaindeadcode/internal/hooks"

func main() {}
`)
	writeFile(t, filepath.Join(dir, "internal", "hooks", "hooks.go"), `package hooks

import _ "unsafe"

//go:linkname linkedRuntimeHook example.com/toolchaindeadcode/internal/runtime.linkedRuntimeHook
func linkedRuntimeHook() {}
`)

	report := runGoToolchainDeadCodeScan(t, dir, true)

	assertFindingRuleAbsent(t, report, "Code Quality", goToolchainDeadCodeRuleID)
}

func TestGoToolchainDeadCodeCoversRealWorldFalsePositiveSurfaces(t *testing.T) {
	dir := t.TempDir()
	writeGoToolchainModule(t, dir)
	writeFile(t, filepath.Join(dir, "cmd", "app", "main.go"), `package main

import (
	_ "example.com/toolchaindeadcode/internal/plugins"
	"example.com/toolchaindeadcode/internal/service"
)

func main() {
	service.Run()
}
`)
	writeFile(t, filepath.Join(dir, "internal", "service", "service.go"), `package service

var callbacks = map[string]func(){
	"startup": startupHook,
}

func Run() {
	registerCallback(callbackHook)
	callbacks["startup"]()
}

func PublicAPI() string { return "public" }

func callbackHook() {}

func startupHook() {}

func registerCallback(fn func()) { fn() }
`)
	writeFile(t, filepath.Join(dir, "internal", "plugins", "plugins.go"), `package plugins

func init() {
	register("demo", pluginHook)
}

func pluginHook() {}

func register(name string, hook func()) { hook() }
`)
	writeFile(t, filepath.Join(dir, "internal", "generated", "generated.go"), `// Code generated by fixture. DO NOT EDIT.
package generated

func GeneratedHook() {}
`)
	writeFile(t, filepath.Join(dir, "vendor", "example.com", "legacy", "legacy.go"), `package legacy

func VendorOnly() {}
`)
	writeFile(t, filepath.Join(dir, "internal", "service", "service_test.go"), `package service

func helperOnlyForTests() {}
`)
	writeFile(t, filepath.Join(dir, "internal", "orphan", "orphan.go"), `package orphan

func cleanup() {}
`)

	report := runGoToolchainDeadCodeScan(t, dir, true)

	assertToolchainDeadCodeFindingPath(t, report, "internal/orphan/orphan.go")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "internal/service/service.go")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "internal/plugins/plugins.go")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "internal/generated/generated.go")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "vendor/example.com/legacy/legacy.go")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "internal/service/service_test.go")
}

func writeGoToolchainModule(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/toolchaindeadcode\n\ngo 1.23.0\n")
}

func runGoToolchainDeadCodeScan(t *testing.T, dir string, enabled bool) codeguard.Report {
	t.Helper()
	writeGoToolchainDeadCodeConfig(t, dir, enabled)
	cfg, err := codeguard.LoadConfigFile(filepath.Join(dir, "codeguard.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func writeGoToolchainDeadCodeConfig(t *testing.T, dir string, enabled bool) {
	t.Helper()
	deadCodeBlock := ""
	if enabled {
		deadCodeBlock = `
    dead_code:
      enabled: true
      mode: toolchain
      level: warn
      include_tests: false
      go:
        packages:
          - ./...
        entrypoints:
          - ./cmd/app
        linker: true
`
	}
	writeFile(t, filepath.Join(dir, "codeguard.yaml"), `name: go-toolchain-dead-code-test
targets:
  - name: repo
    path: .
    language: go
    entrypoints:
      - ./cmd/app
checks:
  quality: true
  design: false
  security: false
  prompts: false
  ci: false
  performance: false
  supply_chain: false
  delivery: false
  reliability: false
  data: false
  observability: false
  operations: false
  change: false
  contracts: false
  context: false
  quality_rules:
    ai_checks:
      dead_code: false
`+deadCodeBlock+`output:
  format: text
`)
}

func assertToolchainDeadCodeFindingPath(t *testing.T, report codeguard.Report, wantPath string) {
	t.Helper()
	for _, finding := range toolchainDeadCodeFindings(report) {
		if filepath.ToSlash(finding.Path) == wantPath {
			return
		}
	}
	t.Fatalf("missing %s finding at %q; got %s", goToolchainDeadCodeRuleID, wantPath, formatFindings(report))
}

func assertToolchainDeadCodeFindingPathAbsent(t *testing.T, report codeguard.Report, path string) {
	t.Helper()
	for _, finding := range toolchainDeadCodeFindings(report) {
		if filepath.ToSlash(finding.Path) == path {
			t.Fatalf("unexpected %s finding at %q: %+v", goToolchainDeadCodeRuleID, path, finding)
		}
	}
}

func toolchainDeadCodeFindings(report codeguard.Report) []codeguard.Finding {
	var findings []codeguard.Finding
	for _, section := range report.Sections {
		if section.Name != "Code Quality" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == goToolchainDeadCodeRuleID {
				findings = append(findings, finding)
			}
		}
	}
	return findings
}

func formatFindings(report codeguard.Report) string {
	var out strings.Builder
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			out.WriteString(section.Name)
			out.WriteString(" ")
			out.WriteString(finding.RuleID)
			out.WriteString(" ")
			out.WriteString(finding.Path)
			out.WriteString(":")
			out.WriteString(finding.Message)
			out.WriteString("\n")
		}
	}
	if out.Len() == 0 {
		return "<none>"
	}
	return out.String()
}
