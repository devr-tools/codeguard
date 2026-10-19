package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
)

// houseStylePolicyRules are the rules that only fire because of a repository
// convention codeguard cannot know applies. A scan with no config file must
// never report them: the zero-config path resolves the conservative defaults,
// not the opinionated config `codeguard init` writes.
var houseStylePolicyRules = []string{
	"ci.test-file-location",    // ci_rules.allowed_test_paths
	"ci.workflow-content",      // ci_rules.workflow_content_rules
	"ci.required-file",         // ci_rules.required_{workflow,release}_files, required_automation_paths
	"ci.missing-required-gate", // ci_rules.required_gates
	"design.service-import-internal",
	"design.service-import-cmd",
	"design.cmd-through-internal-cli",
}

// writeZeroConfigFixture lays down a small Go repository that trips every
// house-style rule if the starter config's policy is in effect: tests sit
// beside the source rather than under tests/, pkg/ imports internal/, the
// workflow does not contain `go test ./...`, and there is no Makefile and no
// .goreleaser.yaml.
func writeZeroConfigFixture(t *testing.T, root string) {
	t.Helper()
	writeScanTestFile(t, filepath.Join(root, "go.mod"), "module example.com/zeroconfig\n\ngo 1.23.0\n")
	writeScanTestFile(t, filepath.Join(root, "internal", "calc", "calc.go"),
		"package calc\n\n// Add returns the sum of a and b.\nfunc Add(a int, b int) int { return a + b }\n")
	// Co-located test file: idiomatic Go, and a ci.test-file-location fail
	// under `allowed_test_paths: ["tests/**"]`.
	writeScanTestFile(t, filepath.Join(root, "internal", "calc", "calc_test.go"),
		"package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"bad sum\")\n\t}\n}\n")
	// pkg/ reaching into internal/ is ordinary Go, and a
	// design.service-import-internal fail under the layering rules.
	writeScanTestFile(t, filepath.Join(root, "pkg", "api", "api.go"),
		"package api\n\nimport \"example.com/zeroconfig/internal/calc\"\n\n// Sum adds two numbers.\nfunc Sum(a int, b int) int { return calc.Add(a, b) }\n")
	// A workflow that tests via make, not `go test ./...`: a
	// ci.workflow-content fail under the starter's content markers. The action
	// ref is SHA-pinned so ci.mutable-deployment-reference stays out of it.
	writeScanTestFile(t, filepath.Join(root, ".github", "workflows", "ci.yml"),
		"name: ci\non: [push]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@8f3c2b1a4d5e6f7890abc1234567890abc123456\n      - run: make check\n")
	writeScanTestFile(t, filepath.Join(root, "README.md"), "# Zero config\n\nRun `make check`.\n")
	writeScanTestFile(t, filepath.Join(root, "AGENTS.md"), "# Agent Notes\n\n## Build & test\n- `make check` runs the suite.\n")
}

type zeroConfigReport struct {
	Sections []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Findings []struct {
			RuleID string `json:"rule_id"`
			Level  string `json:"level"`
		} `json:"findings"`
	} `json:"sections"`
}

func runZeroConfigScan(t *testing.T, args ...string) zeroConfigReport {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Run from a directory with no codeguard config, so resolveConfigPath
	// cannot discover one in "." or ".codeguard" and the fallback is the path
	// actually under test.
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	writeZeroConfigFixture(t, filepath.Join(dir, "repo"))

	var stdout, stderr bytes.Buffer
	full := append([]string{"scan", "-folder", "repo", "-format", "json"}, args...)
	cli.Run(full, strings.NewReader(""), &stdout, &stderr)

	var report zeroConfigReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\nstdout = %s\nstderr = %s", err, stdout.String(), stderr.String())
	}
	if len(report.Sections) == 0 {
		t.Fatalf("zero-config scan reported no sections; stderr = %s", stderr.String())
	}
	return report
}

// A scan with no config file must not enforce codeguard's own repository
// conventions. This is the half that catches the original defect: the fallback
// used to return the `codeguard init` starter config.
func TestZeroConfigScanDoesNotEnforceHouseStylePolicy(t *testing.T) {
	report := runZeroConfigScan(t)

	forbidden := map[string]bool{}
	for _, rule := range houseStylePolicyRules {
		forbidden[rule] = true
	}
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if forbidden[finding.RuleID] {
				t.Errorf("zero-config scan reported %s (%s) in section %s: the no-config path must resolve the conservative defaults, not the init starter config", finding.RuleID, finding.Level, section.Name)
			}
		}
	}
}

// The other half: the fix must not be "make the zero-config path find
// nothing". The recommended sections have to actually run and actually report.
func TestZeroConfigScanStillEnablesRecommendedSections(t *testing.T) {
	report := runZeroConfigScan(t)

	got := map[string]bool{}
	total := 0
	for _, section := range report.Sections {
		got[section.ID] = true
		total += len(section.Findings)
	}
	for _, want := range []string{"quality", "design", "security", "prompts", "ci"} {
		if !got[want] {
			t.Errorf("zero-config scan did not run the %q section; sections = %v", want, sectionIDs(report))
		}
	}
	// Agent Context is enabled by scan mode (full scans) rather than by the
	// recommended set, so a full zero-config scan must include it too.
	if !got["context"] {
		t.Errorf("zero-config full scan did not run the agent-context section; sections = %v", sectionIDs(report))
	}
	if total == 0 {
		t.Error("zero-config scan produced no findings at all; the defaults must still detect something, not silently pass")
	}
}

// A profile overlays the zero-config defaults, including the plain-bool
// section activations that applyCheckDefaults used to drop.
func TestZeroConfigScanProfileEnablesItsSections(t *testing.T) {
	report := runZeroConfigScan(t, "-profile", "enterprise")

	got := map[string]bool{}
	for _, section := range report.Sections {
		got[section.ID] = true
	}
	// supply_chain is a plain bool that only the enterprise profile spec sets.
	if !got["supply_chain"] {
		t.Errorf("-profile enterprise did not enable the supply-chain section; sections = %v", sectionIDs(report))
	}
	// delivery is a *bool the profile sets, which already worked; assert it so
	// a regression in either mechanism is caught.
	if !got["delivery"] {
		t.Errorf("-profile enterprise did not enable the delivery section; sections = %v", sectionIDs(report))
	}
	// The profile must not smuggle the house-style policy back in.
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == "ci.test-file-location" {
				t.Errorf("-profile enterprise reported ci.test-file-location; no profile configures a test layout")
			}
		}
	}
}

func TestZeroConfigScanRejectsUnknownProfile(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	writeZeroConfigFixture(t, filepath.Join(dir, "repo"))

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"scan", "-folder", "repo", "-profile", "nope"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 for an unknown profile, got %d; stdout = %s", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), `unknown profile "nope"`) {
		t.Fatalf("expected an unknown-profile error, got %s", stderr.String())
	}
}

func sectionIDs(report zeroConfigReport) []string {
	ids := make([]string, 0, len(report.Sections))
	for _, section := range report.Sections {
		ids = append(ids, section.ID)
	}
	return ids
}
