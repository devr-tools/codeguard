package externalreports

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestImportSARIFCodeQL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codeql.sarif")
	data := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"CodeQL","rules":[{"id":"go/sql-injection","shortDescription":{"text":"SQL injection"}}]}},"results":[{"ruleId":"go/sql-injection","level":"error","message":{"text":"Query built from request data"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"internal/api.go"},"region":{"startLine":42,"startColumn":7}}}]}]}]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	sections, err := Import([]core.ExternalReportConfig{{Path: path, Format: "sarif"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0].Status != core.StatusFail || len(sections[0].Findings) != 1 {
		t.Fatalf("unexpected sections: %#v", sections)
	}
	finding := sections[0].Findings[0]
	if finding.RuleID != "external.codeql.go-sql-injection" || finding.Path != "internal/api.go" || finding.Line != 42 {
		t.Fatalf("unexpected finding: %#v", finding)
	}
	if finding.Metadata["external_rule_id"] != "go/sql-injection" {
		t.Fatalf("missing source rule metadata: %#v", finding.Metadata)
	}
}

func TestImportSARIFDropsUnsafeArtifactPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.sarif")
	data := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"CodeQL"}},"results":[{"ruleId":"x","message":{"text":"issue"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"../secret"},"region":{"startLine":1}}}]}]}]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	sections, err := Import([]core.ExternalReportConfig{{Path: path, Format: "sarif"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := sections[0].Findings[0].Path; got != "" {
		t.Fatalf("unsafe path was retained: %q", got)
	}
}

func TestImportRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "report.sarif")
	if err := os.WriteFile(target, []byte(`{"runs":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.sarif")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Import([]core.ExternalReportConfig{{Path: link, Format: "sarif"}}); err == nil {
		t.Fatal("expected symlink to be rejected")
	}
}
