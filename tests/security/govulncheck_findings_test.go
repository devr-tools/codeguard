package security_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	govulncheckrunner "github.com/devr-tools/codeguard/internal/codeguard/runner/govulncheck"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// govulncheckReport is govulncheck's text output for two advisories. The
// second advisory has no call trace. govulncheck exits 3 when it finds
// vulnerabilities.
const govulncheckReport = `Scanning your code and 42 packages across 3 dependent modules for known vulnerabilities...

Vulnerability #1: GO-2024-2687
    HTTP/2 CONTINUATION flood in net/http
  Found in: golang.org/x/net@v0.20.0
  Fixed in: golang.org/x/net@v0.23.0
    main.go:12:2: example.com/app.serve calls http2.Server.ServeConn

Vulnerability #2: GO-2023-1840
    Unsafe behavior in setuid/setgid binaries in runtime
  Found in: stdlib@go1.20.1
  Fixed in: stdlib@go1.20.5
`

// installGovulncheck puts a fake govulncheck first on PATH that prints stdout
// and exits with the given status.
func installGovulncheck(t *testing.T, stdout string, exitCode string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Fatal("fake govulncheck is a POSIX shell script; this test cannot run on windows")
	}
	binDir := t.TempDir()
	report := filepath.Join(binDir, "report.txt")
	if err := os.WriteFile(report, []byte(stdout), 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}
	script := "#!/bin/sh\ncat '" + report + "'\nexit " + exitCode + "\n"
	if err := os.WriteFile(filepath.Join(binDir, "govulncheck"), []byte(script), 0o755); err != nil { //nolint:gosec // test helper must be executable
		t.Fatalf("write fake govulncheck: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestGovulncheckRunEmitsOneFindingPerVulnerability(t *testing.T) {
	installGovulncheck(t, govulncheckReport, "3")
	findings, err := govulncheckrunner.Run(context.Background(), writeGoModule(t), "", runnersupport.Context{})
	if err != nil {
		t.Fatalf("exit 3 with parsed vulnerabilities must not be an error, got %v", err)
	}
	want := []struct{ message, advisory, pkg, stack string }{
		{
			message:  "Vulnerability #1: GO-2024-2687 found in golang.org/x/net@v0.20.0 fixed in golang.org/x/net@v0.23.0",
			advisory: "GO-2024-2687",
			pkg:      "golang.org/x/net@v0.20.0",
			stack:    "HTTP/2 CONTINUATION flood in net/http -> main.go:12:2: example.com/app.serve calls http2.Server.ServeConn",
		},
		{
			message:  "Vulnerability #2: GO-2023-1840 found in stdlib@go1.20.1 fixed in stdlib@go1.20.5",
			advisory: "GO-2023-1840",
			pkg:      "stdlib@go1.20.1",
			stack:    "Unsafe behavior in setuid/setgid binaries in runtime",
		},
	}
	if len(findings) != len(want) {
		t.Fatalf("got %d findings, want %d: %#v", len(findings), len(want), findings)
	}
	for i, w := range want {
		got := findings[i]
		if got.RuleID != "security.govulncheck" || got.Level != "fail" {
			t.Errorf("finding %d = %s/%s, want security.govulncheck/fail", i, got.RuleID, got.Level)
		}
		if got.Message != w.message {
			t.Errorf("finding %d message = %q, want %q", i, got.Message, w.message)
		}
		if got.Metadata["advisory_id"] != w.advisory || got.Metadata["package"] != w.pkg || got.Metadata["call_stack"] != w.stack {
			t.Errorf("finding %d metadata = %#v, want advisory %s, package %s, call_stack %q", i, got.Metadata, w.advisory, w.pkg, w.stack)
		}
	}
}

func TestGovulncheckWorkspaceEmitsDeduplicatedSortedFindings(t *testing.T) {
	installGovulncheck(t, govulncheckReport, "3")
	findings, diagnostics := govulncheckrunner.RunWorkspace(context.Background(), writeGoModule(t), "", runnersupport.Context{})
	for _, diagnostic := range diagnostics {
		if diagnostic.Level == "fail" {
			t.Fatalf("unexpected failing diagnostic: %#v", diagnostic)
		}
	}
	gotMessages := make([]string, 0, len(findings))
	for _, finding := range findings {
		if finding.RuleID != "security.govulncheck" || finding.Level != "fail" {
			t.Fatalf("finding = %s/%s, want security.govulncheck/fail", finding.RuleID, finding.Level)
		}
		gotMessages = append(gotMessages, finding.Message)
	}
	// Workspace findings are keyed and sorted by advisory id.
	wantMessages := []string{"govulncheck reported GO-2023-1840", "govulncheck reported GO-2024-2687"}
	if len(gotMessages) != len(wantMessages) || gotMessages[0] != wantMessages[0] || gotMessages[1] != wantMessages[1] {
		t.Fatalf("messages = %q, want %q", gotMessages, wantMessages)
	}
	if got := findings[1].Metadata["affected_modules"]; got != "example.com/stderr" {
		t.Fatalf("affected_modules = %q, want example.com/stderr", got)
	}
	if got := findings[1].Metadata["affected_packages"]; got != "golang.org/x/net@v0.20.0" {
		t.Fatalf("affected_packages = %q, want golang.org/x/net@v0.20.0", got)
	}
}

func TestGovulncheckCleanRunEmitsNoFindings(t *testing.T) {
	installGovulncheck(t, "Scanning your code and 12 packages across 1 dependent module for known vulnerabilities...\n\nNo vulnerabilities found.\n", "0")
	findings, err := govulncheckrunner.Run(context.Background(), writeGoModule(t), "", runnersupport.Context{})
	if err != nil {
		t.Fatalf("clean run returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("clean run produced findings: %#v", findings)
	}
	workspaceFindings, diagnostics := govulncheckrunner.RunWorkspace(context.Background(), writeGoModule(t), "", runnersupport.Context{})
	if len(workspaceFindings) != 0 {
		t.Fatalf("clean workspace run produced findings: %#v", workspaceFindings)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Level == "fail" {
			t.Fatalf("clean run produced a failing diagnostic: %#v", diagnostic)
		}
	}
}
