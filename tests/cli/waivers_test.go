package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
	"github.com/devr-tools/codeguard/internal/version"
)

func TestRunWaiversAuditReportsUnusedWaivers(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "waiver-audit",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": true, "prompts": false, "ci": false, "performance": false, "context": false},
  "output": {"format": "text"},
  "cache": {"enabled": false},
  "waivers": [
    {"rule": "security.hardcoded-secret", "path": "missing.go", "reason": "fixed false positive"},
    {"rule": "security.not-a-real-rule", "reason": "old rule id"}
  ]
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"waivers", "audit", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"Waiver audit: 2 configured, 0 active, 1 unused, 0 expired, 1 unknown rule",
		"[WARN] waiver:0 security.hardcoded-secret path=missing.go: did not match any finding",
		"reason=\"fixed false positive\"",
		"[WARN] waiver:1 security.not-a-real-rule: references a rule that is not enabled in the current catalog",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunWaiversAuditJSONOutput(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "waiver-audit-json",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": true, "prompts": false, "ci": false, "performance": false, "context": false},
  "output": {"format": "text"},
  "cache": {"enabled": false},
  "waivers": [{"rule": "security.hardcoded-secret", "path": "missing.go"}]
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"waivers", "audit", "-config", configPath, "-format", "json"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "unused"`) {
		t.Fatalf("unexpected JSON output:\n%s", stdout.String())
	}
}

func TestRunWaiversAuditNoWaivers(t *testing.T) {
	dir := t.TempDir()
	configPath := writeWaiversAuditConfig(t, dir, "")

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"waivers", "audit", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "Waiver audit: no configured waivers" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRunWaiversAuditActiveWaiversPass(t *testing.T) {
	dir := t.TempDir()
	writeWaiversAuditSecret(t, filepath.Join(dir, "secrets.go"))
	configPath := writeWaiversAuditConfig(t, dir, `{"rule": "security.hardcoded-secret", "path": "secrets.go"}`)

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"waivers", "audit", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"Waiver audit: 1 configured, 1 active, 0 unused, 0 expired, 0 unknown rule",
		"[PASS] all waivers matched at least one finding in this scan",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[WARN]") {
		t.Fatalf("did not expect warnings for active waiver:\n%s", out)
	}
}

func TestRunWaiversAuditRejectsUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	configPath := writeWaiversAuditConfig(t, dir, "")

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"waivers", "audit", "-config", configPath, "-format", "sarif"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout = %s", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), `unsupported waiver audit format "sarif"`) {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunWaiversAuditShowsStaleAfterUpgradeEvidence(t *testing.T) {
	originalVersion := version.Number
	t.Cleanup(func() { version.Number = originalVersion })

	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secrets.go")
	writeWaiversAuditSecret(t, secretPath)
	configPath := writeWaiversAuditHistoryConfig(t, dir)

	version.Number = "1.0.0"
	var firstStdout, firstStderr bytes.Buffer
	if code := cli.Run([]string{"waivers", "audit", "-config", configPath}, strings.NewReader(""), &firstStdout, &firstStderr); code != 0 {
		t.Fatalf("first audit exit code = %d, stderr = %s", code, firstStderr.String())
	}

	if err := os.WriteFile(secretPath, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	version.Number = "1.1.0"
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"waivers", "audit", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"Upgrade evidence: 1 stale after upgrade, 0 inconclusive",
		"matched a finding on the previous CodeGuard version and no longer matches under the current version",
		"matched 1 finding(s) on CodeGuard 1.0.0 and 0 on 1.1.0",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func writeWaiversAuditConfig(t *testing.T, dir string, waivers string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(waivers) != "" {
		waivers = "\n    " + waivers + "\n  "
	}
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "waiver-audit-helper",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": true, "prompts": false, "ci": false, "performance": false, "context": false},
  "output": {"format": "text"},
  "cache": {"enabled": false},
  "waivers": [` + waivers + `]
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func writeWaiversAuditHistoryConfig(t *testing.T, dir string) string {
	t.Helper()
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "waiver-audit-history",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": true, "prompts": false, "ci": false, "performance": false, "context": false},
  "output": {"format": "text"},
  "cache": {"path": "` + filepath.Join(dir, "cache", "scan.json") + `"},
  "waivers": [{"rule": "security.hardcoded-secret", "path": "secrets.go"}]
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func writeWaiversAuditSecret(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(`package main

func main() {
	var api_key = "Zx9Qw3Rt7Yu1Io5P"
	_ = api_key
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
}
