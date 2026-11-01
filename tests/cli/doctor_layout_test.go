package cli_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeDoctorLayoutConfig writes a config whose single Go target is dir, with
// the given entrypoints JSON list and checks JSON object body.
func writeDoctorLayoutConfig(t *testing.T, dir string, entrypoints string, checks string) string {
	t.Helper()
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "doctor-layout",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go", "entrypoints": ` + entrypoints + `}],
  "checks": {` + checks + `},
  "output": {"format": "text"},
  "cache": {"path": "` + filepath.Join(dir, "cache", "scan.json") + `"}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func writeDoctorFile(t *testing.T, path string, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

const doctorChecksOff = `"quality": false, "design": false, "security": false, "prompts": false, "ci": false`

func TestDoctorFailsOnMissingEntrypoint(t *testing.T) {
	dir := t.TempDir()
	writeDoctorFile(t, filepath.Join(dir, "cmd", "app", "main.go"), "package main\n\nfunc main() {}\n", 0o644)
	configPath := writeDoctorLayoutConfig(t, dir, `["cmd/app", "cmd/gone", "cmd/*/main.go"]`, doctorChecksOff)

	code, stdout, stderr := runCLI(t, "doctor", "-config", configPath)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 for a missing entrypoint; stdout=%s stderr=%s", code, stdout, stderr)
	}
	want := "[FAIL] entrypoints:repo: 1 of 3 configured entrypoint(s) do not exist under " + dir + ": cmd/gone"
	if !strings.Contains(stdout, want) {
		t.Fatalf("missing %q in:\n%s", want, stdout)
	}
}

func TestDoctorPassesWhenEntrypointsExist(t *testing.T) {
	dir := t.TempDir()
	writeDoctorFile(t, filepath.Join(dir, "cmd", "app", "main.go"), "package main\n\nfunc main() {}\n", 0o644)
	configPath := writeDoctorLayoutConfig(t, dir, `["cmd/app"]`, doctorChecksOff)

	code, stdout, _ := runCLI(t, "doctor", "-config", configPath)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, "[PASS] entrypoints:repo: all 1 configured entrypoint(s) exist") {
		t.Fatalf("expected entrypoint pass, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "language:repo") {
		t.Fatalf("matching language should not warn, got:\n%s", stdout)
	}
}

func TestDoctorWarnsOnLanguageMismatch(t *testing.T) {
	dir := t.TempDir()
	writeDoctorFile(t, filepath.Join(dir, "app", "main.py"), "print('hi')\n", 0o644)
	// Go sources only inside excluded trees must not count.
	writeDoctorFile(t, filepath.Join(dir, "node_modules", "dep", "x.go"), "package dep\n", 0o644)
	writeDoctorFile(t, filepath.Join(dir, ".hidden", "y.go"), "package hidden\n", 0o644)
	configPath := writeDoctorLayoutConfig(t, dir, `[]`, doctorChecksOff)

	code, stdout, stderr := runCLI(t, "doctor", "-config", configPath)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (language mismatch warns); stdout=%s stderr=%s", code, stdout, stderr)
	}
	for _, want := range []string{
		`[WARN] language:repo: target language "go" has no matching source files (.go)`,
		"found .py files instead",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %q in:\n%s", want, stdout)
		}
	}
}

// installFakeGovulncheck puts a shell-script govulncheck first on PATH.
func installFakeGovulncheck(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake govulncheck is a POSIX shell script")
	}
	binDir := t.TempDir()
	writeDoctorFile(t, filepath.Join(binDir, "govulncheck"), "#!/bin/sh\n"+script, 0o755)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestDoctorRunsGovulncheckAndReportsStderr(t *testing.T) {
	installFakeGovulncheck(t, `echo "govulncheck: loading packages: go: go.mod requires go >= 1.99 (running go 1.23; GOTOOLCHAIN=local)" >&2
exit 1
`)
	dir := t.TempDir()
	writeDoctorFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n", 0o644)
	configPath := writeDoctorLayoutConfig(t, dir, `[]`,
		`"quality": false, "design": false, "security": true, "prompts": false, "ci": false, "security_rules": {"govulncheck_mode": "required"}`)

	code, stdout, stderr := runCLI(t, "doctor", "-config", configPath)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 when required govulncheck cannot run; stdout=%s stderr=%s", code, stdout, stderr)
	}
	for _, want := range []string{
		"[FAIL] govulncheck: govulncheck -version failed: exit status 1",
		"go.mod requires go >= 1.99 (running go 1.23; GOTOOLCHAIN=local)",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %q in:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "[PASS] govulncheck") {
		t.Fatalf("govulncheck must not pass when it cannot run:\n%s", stdout)
	}
}

func TestDoctorGovulncheckProbeOutcomes(t *testing.T) {
	cases := []struct {
		name     string
		script   string
		wantLine string
	}{
		{
			name:     "working tool passes",
			script:   "printf 'Go: go1.23.4\\nScanner: govulncheck@v1.3.0\\n'\n",
			wantLine: "[PASS] govulncheck: govulncheck runs (govulncheck@v1.3.0)",
		},
		{
			name:     "tool without a Go toolchain warns",
			script:   "printf 'Scanner: govulncheck@v1.3.0\\n'\n",
			wantLine: "[WARN] govulncheck: govulncheck runs (govulncheck@v1.3.0) but reports no Go toolchain",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installFakeGovulncheck(t, tc.script)
			dir := t.TempDir()
			writeDoctorFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n", 0o644)
			configPath := writeDoctorLayoutConfig(t, dir, `[]`,
				`"quality": false, "design": false, "security": true, "prompts": false, "ci": false, "security_rules": {"govulncheck_mode": "auto"}`)
			code, stdout, _ := runCLI(t, "doctor", "-config", configPath)
			if code != 0 {
				t.Fatalf("exit = %d, want 0; stdout=%s", code, stdout)
			}
			if !strings.Contains(stdout, tc.wantLine) {
				t.Fatalf("missing %q in:\n%s", tc.wantLine, stdout)
			}
		})
	}
}
