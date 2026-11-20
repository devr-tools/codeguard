package report_test

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// A clean run has no results, and SARIF 2.1.0 requires results to be an
// array: GitHub code scanning rejects "results": null.
func TestSARIFCleanRunEmitsEmptyResultsArray(t *testing.T) {
	report := codeguard.Report{
		GeneratedAt: "2026-11-01T00:00:00Z",
		Sections:    []codeguard.SectionResult{{ID: "quality", Name: "Quality", Status: codeguard.StatusPass}},
	}
	raw := renderSARIF(t, report)
	var doc struct {
		Runs []map[string]json.RawMessage `json:"runs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse sarif: %v", err)
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("expected one run, got %d", len(doc.Runs))
	}
	if got := strings.TrimSpace(string(doc.Runs[0]["results"])); got != "[]" {
		t.Fatalf("clean run results = %s, want []", got)
	}
	if errs := loadSARIFSchema(t).validateSARIF(t, raw); len(errs) > 0 {
		t.Fatalf("clean-run SARIF violates the 2.1.0 schema:\n%s", strings.Join(errs, "\n"))
	}
}

// uriReferenceChars is the RFC 3986 character set a URI reference may use
// outside percent-escapes; a raw space, '#' in a path, or a stray '%' is not
// a valid artifactLocation.uri.
func assertURIReference(t *testing.T, uri string) {
	t.Helper()
	for _, r := range uri {
		if r > 0x7e || r <= 0x20 || strings.ContainsRune(`"<>\^`+"`{|}#", r) {
			t.Errorf("uri %q contains %q, which is not allowed unescaped in a URI path", uri, r)
		}
	}
	if _, err := url.Parse(uri); err != nil {
		t.Errorf("uri %q does not parse: %v", uri, err)
	}
}

func uriOf(t *testing.T, location sarifLocation) string {
	t.Helper()
	if location.PhysicalLocation == nil {
		t.Fatal("location has no physicalLocation")
	}
	return location.PhysicalLocation.ArtifactLocation.URI
}

// Paths with characters that are special in URIs are percent-encoded per
// segment, keep their separators, decode back to the original path, and the
// document still validates against the schema.
func TestSARIFArtifactURIsArePercentEncoded(t *testing.T) {
	path := "dir with space/50%/a#b.go"
	report := findingsReport("security",
		codeguard.Finding{RuleID: "security.taint.go", Level: "fail", Path: path, Line: 5,
			Message: "tainted data from os.Getenv (line 3) reaches exec.Command (line 5) via cmd -> exec.Command"},
		codeguard.Finding{RuleID: "quality.gofmt", Level: "warn", Path: "odd:name.go", Line: 1, Message: "unformatted"},
	)
	report.Sections[0].Diagnostics = []core.Diagnostic{
		{ID: "scan.file-read", Level: "warn", Kind: "infrastructure", Message: "could not read", Path: path, Operational: true},
	}
	raw := renderSARIF(t, report)
	doc := parseSARIF(t, raw)
	want := "dir%20with%20space/50%25/a%23b.go"

	taint := resultByRule(t, doc, "security.taint.go")
	if got := uriOf(t, taint.Locations[0]); got != want {
		t.Fatalf("finding uri = %q, want %q", got, want)
	}
	if len(taint.CodeFlows) == 0 {
		t.Fatal("taint finding lost its code flow")
	}
	for _, step := range taint.CodeFlows[0].ThreadFlows[0].Locations {
		if got := uriOf(t, step.Location); got != want {
			t.Fatalf("code flow step uri = %q, want %q", got, want)
		}
	}
	if got := uriOf(t, resultByRule(t, doc, "scan.file-read").Locations[0]); got != want {
		t.Fatalf("diagnostic uri = %q, want %q", got, want)
	}
	// A colon in the first segment of a relative reference would parse as a
	// scheme, so it is escaped too.
	if got := uriOf(t, resultByRule(t, doc, "quality.gofmt").Locations[0]); got != "odd%3Aname.go" {
		t.Fatalf("colon path uri = %q, want odd%%3Aname.go", got)
	}
	for _, result := range doc.Runs[0].Results {
		for _, location := range result.Locations {
			uri := uriOf(t, location)
			assertURIReference(t, uri)
			if decoded, err := url.PathUnescape(uri); err != nil || (result.RuleID != "quality.gofmt" && decoded != path) {
				t.Errorf("uri %q decodes to %q (err %v), want %q", uri, decoded, err, path)
			}
		}
	}
	if errs := loadSARIFSchema(t).validateSARIF(t, raw); len(errs) > 0 {
		t.Fatalf("SARIF with encoded paths violates the 2.1.0 schema:\n%s", strings.Join(errs, "\n"))
	}
}

// Some diagnostics carry absolute filesystem paths (a target root, a Go
// module directory). One under the working directory, which is the checkout
// in the GitHub Action, becomes repository-relative; any other becomes a
// file:// URI rather than a bare absolute path.
func TestSARIFAbsoluteDiagnosticPathsBecomeValidURIs(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(wd, "sub dir", "go.mod")
	outside := filepath.Join(filepath.VolumeName(wd)+string(filepath.Separator), "codeguard-outside-root", "mod #1")
	report := findingsReport("security")
	report.Sections[0].Diagnostics = []core.Diagnostic{
		{ID: "scan.govulncheck.module-status", Level: "info", Kind: "scan_status", Message: "inside", Path: inside},
		{ID: "scan.target-walk", Level: "fail", Kind: "infrastructure", Message: "outside", Path: outside, Operational: true},
	}
	raw := renderSARIF(t, report)
	doc := parseSARIF(t, raw)

	if got := uriOf(t, resultByRule(t, doc, "scan.govulncheck.module-status").Locations[0]); got != "sub%20dir/go.mod" {
		t.Fatalf("in-checkout absolute path uri = %q, want sub%%20dir/go.mod", got)
	}
	got := uriOf(t, resultByRule(t, doc, "scan.target-walk").Locations[0])
	if !strings.HasPrefix(got, "file:///") || !strings.HasSuffix(got, "/codeguard-outside-root/mod%20%231") {
		t.Fatalf("out-of-checkout absolute path uri = %q, want a file:/// URI ending in /codeguard-outside-root/mod%%20%%231", got)
	}
	parsed, err := url.Parse(got)
	if err != nil || parsed.Scheme != "file" {
		t.Fatalf("uri %q is not a file URI: %v", got, err)
	}
	assertURIReference(t, got)
	if errs := loadSARIFSchema(t).validateSARIF(t, raw); len(errs) > 0 {
		t.Fatalf("SARIF with absolute diagnostic paths violates the 2.1.0 schema:\n%s", strings.Join(errs, "\n"))
	}
}
