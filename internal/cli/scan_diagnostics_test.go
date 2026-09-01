package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestScanMonitorReportsProgressAndMemoryWithoutPollutingReportOutput(t *testing.T) {
	var diagnostics bytes.Buffer
	monitor := newScanMonitor(&diagnostics, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))
	monitor.writeStarted()
	monitor.writeHeartbeat(time.Date(2026, 9, 1, 12, 1, 30, 0, time.UTC), 384<<20, 8<<30)
	monitor.writeSectionComplete(service.SectionResult{Name: "Code Quality", Status: service.StatusPass})

	got := diagnostics.String()
	for _, want := range []string{
		"scan started",
		"scan in progress: elapsed=1m30s heap=384 MiB memory_limit=8192 MiB",
		"completed Code Quality: pass",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("diagnostics %q do not contain %q", got, want)
		}
	}
}

func TestWriteHeapProfileCreatesUsableProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scan.heap.pprof")
	if err := writeHeapProfile(path); err != nil {
		t.Fatalf("write heap profile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read heap profile: %v", err)
	}
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		t.Fatalf("heap profile does not have gzip header: %x", data[:min(len(data), 8)])
	}
}

func TestRunScanStreamsProgressAndWritesRequestedHeapProfile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	contextDisabled := false
	configPath := filepath.Join(root, "codeguard.json")
	cfg := service.Config{
		Name:    "diagnostic-scan",
		Targets: []service.TargetConfig{{Name: "repo", Path: root, Language: "go"}},
		Checks: service.CheckConfig{
			Quality: true,
			Context: &contextDisabled,
		},
		Output: service.OutputConfig{Format: "github"},
	}
	if err := service.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	profilePath := filepath.Join(root, "scan.heap.pprof")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"scan", "-config", configPath, "-format", "github", "-memprofile", profilePath},
		strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("scan exit code = %d, want 0; stderr: %s", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "scan started") || !strings.Contains(stderr.String(), "completed Code Quality") {
		t.Fatalf("stderr does not contain streamed progress: %q", stderr.String())
	}
	if strings.Contains(stdout.String(), "scan started") {
		t.Fatalf("machine-readable stdout was polluted by progress: %q", stdout.String())
	}
	if info, err := os.Stat(profilePath); err != nil {
		t.Fatalf("stat heap profile: %v", err)
	} else if info.Size() == 0 {
		t.Fatal("heap profile is empty")
	}
}

func TestRunScanStillWritesReportWhenHeapProfileCannotBeWritten(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	contextDisabled := false
	configPath := filepath.Join(root, "codeguard.json")
	cfg := service.Config{
		Name:    "profile-failure-scan",
		Targets: []service.TargetConfig{{Name: "repo", Path: root, Language: "go"}},
		Checks: service.CheckConfig{
			Quality: true,
			Context: &contextDisabled,
		},
		Output: service.OutputConfig{Format: "github"},
	}
	if err := service.WriteConfigFile(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	profilePath := filepath.Join(root, "missing", "scan.heap.pprof")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"scan", "-config", configPath, "-format", "github", "-memprofile", profilePath},
		strings.NewReader(""), &stdout, &stderr)
	if exitCode == 0 {
		t.Fatal("scan with unwritable requested heap profile returned success")
	}
	if !strings.Contains(stdout.String(), "::notice title=CodeGuard") {
		t.Fatalf("completed scan report was not written: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "heap profile") {
		t.Fatalf("heap profile failure was not diagnosed: %q", stderr.String())
	}
}
