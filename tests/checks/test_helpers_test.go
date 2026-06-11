package checks_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertSectionStatus(t *testing.T, report codeguard.Report, name string, want string) {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name == name {
			if string(section.Status) != want {
				t.Fatalf("%s status = %q, want %q", name, section.Status, want)
			}
			return
		}
	}
	t.Fatalf("section %q not found", name)
}

func assertSectionFindingCountAtLeast(t *testing.T, report codeguard.Report, name string, min int) {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name == name {
			if len(section.Findings) < min {
				t.Fatalf("%s findings = %d, want at least %d", name, len(section.Findings), min)
			}
			return
		}
	}
	t.Fatalf("section %q not found", name)
}

func strippedANSI(value string) string {
	return strings.NewReplacer(
		"\x1b[38;2;10;18;60m", "",
		"\x1b[38;2;37;169;255m", "",
		"\x1b[31m", "",
		"\x1b[0m", "",
	).Replace(value)
}

func assertTextReportFormatting(t *testing.T, buf *bytes.Buffer) {
	t.Helper()
	ansiStripped := strippedANSI(buf.String())
	if !strings.Contains(ansiStripped, "⢀⣠⠤⠶⠲⠦⢤⣀") {
		t.Fatalf("expected logo from img/codeguard.txt, got: %s", ansiStripped)
	}
	if !strings.Contains(buf.String(), "\x1b[38;2;37;169;255m") {
		t.Fatalf("expected blue brand color in header, got: %q", buf.String())
	}
	if strings.Contains(ansiStripped, "\ncodeguard\n") {
		t.Fatalf("expected asset-backed logo without duplicate wordmark, got: %s", ansiStripped)
	}
	if !strings.Contains(buf.String(), "| Section ") {
		t.Fatalf("expected summary table, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "\x1b[31mFAIL\x1b[0m") {
		t.Fatalf("expected colored fail status, got: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "❌ FAIL") {
		t.Fatalf("expected fail icon, got: %q", buf.String())
	}
	if !strings.Contains(ansiStripped, "- [FAIL] security.hardcoded-secret") {
		t.Fatalf("expected stacked finding title with badge, got: %s", ansiStripped)
	}
	if !strings.Contains(ansiStripped, "at: config.go:3") {
		t.Fatalf("expected finding location line, got: %s", ansiStripped)
	}
	if !strings.Contains(ansiStripped, "rule: security.hardcoded-secret") {
		t.Fatalf("expected finding rule line, got: %s", ansiStripped)
	}
	if strings.Contains(ansiStripped, "severity: fail") {
		t.Fatalf("expected severity line to be removed, got: %s", ansiStripped)
	}
}

func assertPlainTextReportFormatting(t *testing.T, buf *bytes.Buffer) {
	t.Helper()
	if strings.Contains(buf.String(), "\x1b[31m") {
		t.Fatalf("expected NO_COLOR output to omit ANSI escapes, got: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "⢀⣠⠤⠶⠲⠦⢤⣀") {
		t.Fatalf("expected plain output to use img/codeguard.txt logo, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "- [FAIL] security.hardcoded-secret") {
		t.Fatalf("expected plain output to include finding badge, got: %s", buf.String())
	}
}
