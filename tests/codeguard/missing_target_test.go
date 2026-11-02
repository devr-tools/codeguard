package codeguard_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// missingTargetConfig runs only Code Quality, the section the original report
// showed passing over a target that did not exist.
func missingTargetConfig(path string) codeguard.Config {
	disabled := false
	cfg := codeguard.ExampleConfig()
	cfg.Name = "missing-target"
	cfg.Targets = []codeguard.TargetConfig{{Name: "app", Path: path, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SupplyChain = false
	cfg.Checks.Context = &disabled
	cfg.Checks.Performance = &disabled
	cfg.Cache.Enabled = &disabled
	return cfg
}

func TestRunRejectsNonexistentTargetPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	report, err := codeguard.Run(context.Background(), missingTargetConfig(missing))
	if err == nil {
		t.Fatalf("Run over a nonexistent target succeeded with sections %+v; want an error", report.Sections)
	}
	if !strings.Contains(err.Error(), "does not exist") || !strings.Contains(err.Error(), `"app"`) {
		t.Fatalf("error = %q, want it to name target \"app\" and say the path does not exist", err)
	}
}

func TestRunRejectsNonDirectoryTargetPath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err := codeguard.Run(context.Background(), missingTargetConfig(file))
	if err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("Run over a file target error = %v, want a not-a-directory error", err)
	}
}

func TestValidateConfigStaysPureForMissingTargetPath(t *testing.T) {
	// ValidateConfig checks shape only: SDK callers validate in-memory configs
	// whose targets need not exist where the validation runs.
	cfg := missingTargetConfig(filepath.Join(t.TempDir(), "does-not-exist"))
	if err := codeguard.ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig = %v, want nil (it must not touch the filesystem)", err)
	}
	if err := codeguard.ValidateTargetPaths(cfg); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("ValidateTargetPaths = %v, want a does-not-exist error", err)
	}
}

func TestValidateTargetPathsAcceptsExistingDirectory(t *testing.T) {
	if err := codeguard.ValidateTargetPaths(missingTargetConfig(t.TempDir())); err != nil {
		t.Fatalf("ValidateTargetPaths(existing dir) = %v, want nil", err)
	}
}

func skipIfPermissionsUnenforced(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not enforced on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores permission bits")
	}
}

func findDiagnostic(section codeguard.SectionResult, id string) (core.Diagnostic, bool) {
	for _, diagnostic := range section.Diagnostics {
		if diagnostic.ID == id {
			return diagnostic, true
		}
	}
	return core.Diagnostic{}, false
}

func TestRunFailsSectionWhenTargetWalkErrors(t *testing.T) {
	skipIfPermissionsUnenforced(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(locked, "hidden.go"), []byte("package locked\n"), 0o600); err != nil {
		t.Fatalf("write hidden.go: %v", err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) })

	report, err := codeguard.Run(context.Background(), missingTargetConfig(root))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	quality := findSection(t, report, "Code Quality")
	diagnostic, ok := findDiagnostic(quality, "scan.target-walk")
	if !ok {
		t.Fatalf("quality diagnostics = %+v, want scan.target-walk", quality.Diagnostics)
	}
	if diagnostic.Level != "fail" || !diagnostic.Operational {
		t.Fatalf("walk diagnostic = %+v, want an operational fail", diagnostic)
	}
	if quality.Status != codeguard.StatusFail {
		t.Fatalf("quality status = %q, want fail when the target walk errored", quality.Status)
	}
	if report.Summary.FailedSections == 0 {
		t.Fatalf("summary = %+v, want a failed section", report.Summary)
	}
}

func TestRunReportsUnreadableListedFile(t *testing.T) {
	skipIfPermissionsUnenforced(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	unreadable := filepath.Join(root, "unreadable.go")
	if err := os.WriteFile(unreadable, []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write unreadable.go: %v", err)
	}
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) })

	report, err := codeguard.Run(context.Background(), missingTargetConfig(root))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var found *core.Diagnostic
	for _, section := range report.Sections {
		if diagnostic, ok := findDiagnostic(section, "scan.file-read"); ok {
			found = &diagnostic
			if section.Status == codeguard.StatusPass {
				t.Fatalf("section %q passed despite an unreadable file", section.ID)
			}
			break
		}
	}
	if found == nil {
		t.Fatalf("no section reported scan.file-read for an unreadable listed file: %+v", report.Sections)
	}
	if !strings.HasSuffix(found.Path, "unreadable.go") || found.Level != "warn" || !found.Operational {
		t.Fatalf("read diagnostic = %+v, want an operational warn naming unreadable.go", *found)
	}
}
