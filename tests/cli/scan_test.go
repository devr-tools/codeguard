package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
)

func TestRunScanRejectsInvalidMode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := cli.Run([]string{"scan", "-mode", "sideways"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), `invalid scan mode "sideways"`) {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunInteractiveScanUsesPromptedBaseRef(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/interactive\n\ngo 1.23.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	runGit(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {\n\tprintln(\"changed\")\n}\n"), 0o644); err != nil {
		t.Fatalf("rewrite main.go: %v", err)
	}

	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "interactive-scan",
  "targets": [{"name": "repo", "path": ".", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "text"}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	input := strings.NewReader(configPath + "\ndiff\norigin/main\n")

	code := cli.Run([]string{"scan", "-interactive"}, input, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("scan exit code = %d, stderr = %s", code, stderr.String())
	}
	rendered := stripANSI(stdout.String())
	if !strings.Contains(rendered, "Base Ref: origin/main") {
		t.Fatalf("expected prompted base ref in output, got:\n%s", rendered)
	}
}

// writeHintScanConfig writes a minimal all-checks-off scan config; checksJSON
// is the raw JSON of the "checks" object so tests control exactly which keys
// are present (the performance hint keys off key absence, not value).
func writeHintScanConfig(t *testing.T, dir string, checksJSON string) string {
	t.Helper()
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "performance-hint",
  "targets": [{"name": "repo", "path": ".", "language": "go"}],
  "checks": ` + checksJSON + `,
  "output": {"format": "text"}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	return configPath
}

func TestRunScanSuggestsPerformanceSectionWhenKeyAbsent(t *testing.T) {
	dir := t.TempDir()
	configPath := writeHintScanConfig(t, dir,
		`{"quality": false, "design": false, "security": false, "prompts": false, "ci": false}`)

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"scan", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("scan exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "predates the performance check section") {
		t.Fatalf("expected performance upgrade hint for config without the key, got:\n%s", stdout.String())
	}
}

func TestRunScanStaysSilentWhenPerformanceKeyExplicit(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		t.Run("performance_"+value, func(t *testing.T) {
			dir := t.TempDir()
			configPath := writeHintScanConfig(t, dir,
				`{"quality": false, "design": false, "security": false, "prompts": false, "ci": false, "performance": `+value+`}`)

			var stdout, stderr bytes.Buffer
			code := cli.Run([]string{"scan", "-config", configPath}, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("scan exit code = %d, stderr = %s", code, stderr.String())
			}
			if strings.Contains(stdout.String(), "predates the performance check section") {
				t.Fatalf("expected no upgrade hint for explicit performance: %s, got:\n%s", value, stdout.String())
			}
		})
	}
}

func TestRunScanPathFlagScopesFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/folderscan\n\ngo 1.23.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "outside"), 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "good.go"), []byte("package main\n\nfunc good() {}\n"), 0o644); err != nil {
		t.Fatalf("write good.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outside", "bad.go"), []byte("package main\nfunc bad(){println(\"hi\")}\n"), 0o644); err != nil {
		t.Fatalf("write bad.go: %v", err)
	}

	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "folder-scan",
  "targets": [{"name": "repo", "path": ".", "language": "go"}],
  "checks": {"quality": true, "design": false, "security": false, "prompts": false, "ci": false, "performance": false, "context": false},
  "output": {"format": "text"}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"scan", "-config", configPath, "-path", filepath.Join(dir, "sub")}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("scan exit code = %d, stderr = %s\nstdout = %s", code, stderr.String(), stdout.String())
	}
}

func TestRunScanFolderWithoutConfigUsesDefaultProfile(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	writeScanTestFile(t, filepath.Join(dir, "sub", "go.mod"), "module example.com/configless\n\ngo 1.23.0\n")
	writeScanTestFile(t, filepath.Join(dir, "sub", "main.go"), "package main\n\nfunc main() {}\n")
	writeScanTestFile(t, filepath.Join(dir, "sub", "Makefile"), "test:\n\tgo test ./...\n")
	writeScanTestFile(t, filepath.Join(dir, "sub", "README.md"), "# Configless scan\n\nRun `make test`.\n")
	writeScanTestFile(t, filepath.Join(dir, "sub", "AGENTS.md"), "# Agent Notes\n\n## Build & test\n- `make test` runs the unit suite.\n")
	writeScanTestFile(t, filepath.Join(dir, "sub", ".github", "workflows", "ci.yml"), "name: ci\non: [push]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - run: go test ./...\n")

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"scan", "-folder", "sub", "-profile", "startup"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("scan exit code = %d, stderr = %s\nstdout = %s", code, stderr.String(), stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".codeguard")); !os.IsNotExist(err) {
		t.Fatalf("expected configless scan not to create .codeguard cache directory, stat err = %v", err)
	}
}

func TestRunScanFolderWithExplicitMissingConfigStillFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"scan", "-config", filepath.Join(dir, "missing.yaml"), "-folder", filepath.Join(dir, "sub")}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d; stdout = %s", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), "load config:") {
		t.Fatalf("expected load config error, got %s", stderr.String())
	}
}

func writeScanTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
}
