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
		"\x1b[33m", "",
		"\x1b[31m", "",
		"\x1b[0m", "",
	).Replace(value)
}

func assertTextReportFormatting(t *testing.T, buf *bytes.Buffer) {
	t.Helper()
	ansiStripped := strippedANSI(buf.String())
	assertContains(t, ansiStripped, "⢀⣠⠤⠶⠲⠦⢤⣀", "expected logo from img/codeguard.txt")
	assertContains(t, buf.String(), "\x1b[38;2;37;169;255m", "expected blue brand color in header")
	assertNotContains(t, ansiStripped, "\ncodeguard\n", "expected asset-backed logo without duplicate wordmark")
	assertContains(t, buf.String(), "| Section ", "expected summary table")
	assertContains(t, buf.String(), "\x1b[33mWARN\x1b[0m", "expected colored warn status")
	assertContains(t, buf.String(), "⚠️ WARN", "expected warn icon")
	assertContains(t, ansiStripped, "[⚠️ WARN] Code Quality", "expected warn code quality section header")
	assertContains(t, ansiStripped, "\n  Cyclomatic complexity\n", "expected cyclomatic complexity subsection title")
	assertContains(t, ansiStripped, "\n  Dependency direction\n", "expected dependency direction subsection title")
	assertContains(t, ansiStripped, "1. at: tests/checks/test_helpers_test.go:58", "expected numbered finding location line")
	assertContains(t, ansiStripped, "rule: quality.cyclomatic-complexity", "expected finding rule line")
	assertContains(t, ansiStripped, "rule: quality.dependency-direction", "expected dependency direction rule line")
	assertNotContains(t, ansiStripped, "severity: warn", "expected severity line to be removed")
}

func assertPlainTextReportFormatting(t *testing.T, buf *bytes.Buffer) {
	t.Helper()
	assertNotContains(t, buf.String(), "\x1b[31m", "expected NO_COLOR output to omit ANSI escapes")
	assertContains(t, buf.String(), "⢀⣠⠤⠶⠲⠦⢤⣀", "expected plain output to use img/codeguard.txt logo")
	assertContains(t, buf.String(), "\n  Cyclomatic complexity\n", "expected plain output to include grouped subsection title")
}

func assertContains(t *testing.T, text string, needle string, message string) {
	t.Helper()
	if !strings.Contains(text, needle) {
		t.Fatalf("%s, got: %s", message, text)
	}
}

func assertNotContains(t *testing.T, text string, needle string, message string) {
	t.Helper()
	if strings.Contains(text, needle) {
		t.Fatalf("%s, got: %s", message, text)
	}
}
