package checks_test

import (
	"os"
	"path/filepath"
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
