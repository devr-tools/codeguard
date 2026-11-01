package report_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	rulespkg "github.com/devr-tools/codeguard/internal/codeguard/rules"
	runnerchecks "github.com/devr-tools/codeguard/internal/codeguard/runner/checks"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

type sarifDoc struct {
	Runs []struct {
		Tool struct {
			Driver struct {
				Name           string      `json:"name"`
				Version        string      `json:"version"`
				InformationURI string      `json:"informationUri"`
				Rules          []sarifRule `json:"rules"`
			} `json:"driver"`
		} `json:"tool"`
		Invocations []struct {
			ExecutionSuccessful bool `json:"executionSuccessful"`
		} `json:"invocations"`
		Results []sarifResult `json:"results"`
	} `json:"runs"`
}

type sarifRule struct {
	ID               string `json:"id"`
	ShortDescription struct {
		Text string `json:"text"`
	} `json:"shortDescription"`
	FullDescription struct {
		Text string `json:"text"`
	} `json:"fullDescription"`
	Help struct {
		Text     string `json:"text"`
		Markdown string `json:"markdown"`
	} `json:"help"`
	HelpURI              string `json:"helpUri"`
	DefaultConfiguration struct {
		Level string `json:"level"`
	} `json:"defaultConfiguration"`
	Properties struct {
		Tags             []string `json:"tags"`
		Precision        string   `json:"precision"`
		SecuritySeverity string   `json:"security-severity"`
	} `json:"properties"`
}

type sarifLocation struct {
	PhysicalLocation *struct {
		ArtifactLocation struct {
			URI string `json:"uri"`
		} `json:"artifactLocation"`
		Region *struct {
			StartLine   int `json:"startLine"`
			StartColumn int `json:"startColumn"`
		} `json:"region"`
	} `json:"physicalLocation"`
	Message *struct {
		Text string `json:"text"`
	} `json:"message"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Locations []sarifLocation `json:"locations"`
	CodeFlows []struct {
		ThreadFlows []struct {
			Locations []struct {
				Location sarifLocation `json:"location"`
				Kinds    []string      `json:"kinds"`
			} `json:"locations"`
		} `json:"threadFlows"`
	} `json:"codeFlows"`
	PartialFingerprints map[string]string `json:"partialFingerprints"`
}

func renderSARIF(t *testing.T, report codeguard.Report) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := codeguard.WriteReport(&buf, report, "sarif"); err != nil {
		t.Fatalf("write sarif: %v", err)
	}
	return buf.Bytes()
}

func parseSARIF(t *testing.T, raw []byte) sarifDoc {
	t.Helper()
	var doc sarifDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse sarif: %v\n%s", err, raw)
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("expected exactly one run, got %d", len(doc.Runs))
	}
	return doc
}

func findingsReport(sectionID string, findings ...codeguard.Finding) codeguard.Report {
	return codeguard.Report{
		GeneratedAt: "2026-11-01T00:00:00Z",
		Sections: []codeguard.SectionResult{{
			ID: sectionID, Name: sectionID, Status: codeguard.StatusFail, Findings: findings,
		}},
	}
}

func ruleByID(t *testing.T, doc sarifDoc, id string) sarifRule {
	t.Helper()
	for _, rule := range doc.Runs[0].Tool.Driver.Rules {
		if rule.ID == id {
			return rule
		}
	}
	t.Fatalf("rule %s missing from tool.driver.rules", id)
	return sarifRule{}
}

func resultByRule(t *testing.T, doc sarifDoc, id string) sarifResult {
	t.Helper()
	for _, result := range doc.Runs[0].Results {
		if result.RuleID == id {
			return result
		}
	}
	t.Fatalf("no result for rule %s", id)
	return sarifResult{}
}

const goTaintSource = `package main

import (
	"os"
	"os/exec"
)

func main() {
	userCmd := os.Getenv("USER_CMD")
	alias := userCmd
	_ = exec.Command("sh", "-c", alias)
}
`

func scanGoTaintFixture(t *testing.T, dir string) codeguard.Report {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(goTaintSource), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := codeguard.ExampleConfig()
	cfg.Name = "sarif-taint"
	cfg.Targets = []codeguard.TargetConfig{{Name: "app", Path: dir, Language: "go"}}
	cfg.Checks.Security = true
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	disabled := false
	cfg.Cache.Enabled = &disabled
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func containsTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

// A rule that appears in the results must carry everything GitHub code
// scanning renders in its alert panel: descriptions, help text and markdown, a
// help link, a default level, tags, precision, and (for security rules) a
// security-severity score.
func TestSARIFRuleMetadataIsPopulatedFromTheCatalog(t *testing.T) {
	report := findingsReport("security",
		codeguard.Finding{RuleID: "security.taint.go", Level: "fail", Confidence: "high", Message: "tainted data from os.Getenv (line 3) reaches exec.Command (line 5) via cmd -> exec.Command", Path: "main.go", Line: 5, Column: 1},
	)
	report.Sections = append(report.Sections, codeguard.SectionResult{ID: "quality", Name: "Code Quality", Status: codeguard.StatusWarn, Findings: []codeguard.Finding{
		{RuleID: "quality.max-function-lines", Level: "warn", Message: "function too long", Path: "big.go", Line: 10},
	}})
	doc := parseSARIF(t, renderSARIF(t, report))
	catalog := rulespkg.Catalog()

	taint := ruleByID(t, doc, "security.taint.go")
	meta := catalog["security.taint.go"]
	if taint.ShortDescription.Text != meta.Title || taint.FullDescription.Text != meta.Description {
		t.Errorf("descriptions = %q / %q, want catalog title/description", taint.ShortDescription.Text, taint.FullDescription.Text)
	}
	if !strings.Contains(taint.Help.Text, meta.HowToFix) || !strings.Contains(taint.Help.Markdown, meta.HowToFix) {
		t.Errorf("help text/markdown must carry the catalog fix guidance:\n%s\n%s", taint.Help.Text, taint.Help.Markdown)
	}
	if !strings.Contains(taint.Help.Markdown, "https://cwe.mitre.org/data/definitions/78.html") {
		t.Errorf("help markdown should link the CWE definitions:\n%s", taint.Help.Markdown)
	}
	if want := rulespkg.DocsURL + "#rule-security-taint-go"; taint.HelpURI != want {
		t.Errorf("helpUri = %q, want %q", taint.HelpURI, want)
	}
	if taint.DefaultConfiguration.Level != "error" {
		t.Errorf("defaultConfiguration.level = %q, want error", taint.DefaultConfiguration.Level)
	}
	for _, tag := range []string{"security", "external/cwe/cwe-078", "external/cwe/cwe-089", "OWASP:A03:2021"} {
		if !containsTag(taint.Properties.Tags, tag) {
			t.Errorf("security.taint.go tags %v missing %q", taint.Properties.Tags, tag)
		}
	}
	if taint.Properties.Precision != "high" {
		t.Errorf("precision = %q, want high", taint.Properties.Precision)
	}
	if taint.Properties.SecuritySeverity != "9.3" {
		t.Errorf("security-severity = %q, want 9.3", taint.Properties.SecuritySeverity)
	}

	quality := ruleByID(t, doc, "quality.max-function-lines")
	if quality.ShortDescription.Text == "" || quality.HelpURI == "" || quality.Help.Markdown == "" {
		t.Errorf("quality rule metadata incomplete: %+v", quality)
	}
	if containsTag(quality.Properties.Tags, "security") || quality.Properties.SecuritySeverity != "" {
		t.Errorf("non-security rule must not be tagged security or scored: %+v", quality.Properties)
	}
	if !containsTag(quality.Properties.Tags, "quality") {
		t.Errorf("quality rule tags %v missing the section tag", quality.Properties.Tags)
	}
	if quality.Properties.Precision != "medium" {
		t.Errorf("unspecified confidence should report precision medium, got %q", quality.Properties.Precision)
	}
	if quality.DefaultConfiguration.Level != "warning" {
		t.Errorf("quality defaultConfiguration.level = %q, want warning", quality.DefaultConfiguration.Level)
	}
	if got := resultByRule(t, doc, "security.taint.go").Level; got != "error" {
		t.Errorf("fail finding level = %q, want error", got)
	}
	if got := resultByRule(t, doc, "quality.max-function-lines").Level; got != "warning" {
		t.Errorf("warn finding level = %q, want warning", got)
	}
}

var cweID = regexp.MustCompile(`^CWE-[1-9][0-9]*$`)

// Every taint/SSRF engine must name the weaknesses it reports, and every
// Security-section rule needs a CWE so GitHub can group it with other tools'
// alerts. The only exemptions report something without a fixed weakness.
func TestCatalogSecurityAndTaintRulesCarryCWE(t *testing.T) {
	exempt := map[string]bool{
		"security.command-check":  true, // weakness depends on the external tool
		"security.secrets-config": true, // reports codeguard's own config
	}
	for id, meta := range rulespkg.Catalog() {
		for _, cwe := range meta.CWE {
			if !cweID.MatchString(cwe) {
				t.Errorf("%s: malformed CWE id %q", id, cwe)
			}
		}
		taint := strings.Contains(id, "taint") || strings.Contains(id, ".ssrf.") || strings.Contains(id, "untrusted-input-flow")
		if (taint || strings.EqualFold(meta.Section, "security")) && !exempt[id] && len(meta.CWE) == 0 {
			t.Errorf("%s has no CWE mapping; add one in internal/codeguard/rules/catalog_security_cwe.go", id)
		}
	}
}

func TestSARIFTaintRulesEmitCWETags(t *testing.T) {
	for _, id := range []string{"security.taint.go", "security.taint.python", "security.taint.cpp", "security.ssrf.go", "security.typescript.taint-flow"} {
		doc := parseSARIF(t, renderSARIF(t, findingsReport("security", codeguard.Finding{RuleID: id, Level: "fail", Message: "flow", Path: "a.go", Line: 1})))
		rule := ruleByID(t, doc, id)
		var cweTags int
		for _, tag := range rule.Properties.Tags {
			if strings.HasPrefix(tag, "external/cwe/cwe-") {
				cweTags++
			}
		}
		if cweTags == 0 {
			t.Errorf("%s: no external/cwe tags in %v", id, rule.Properties.Tags)
		}
	}
}

// The security-severity mapping is documented in docs/checks.md; this pins it.
func TestSARIFSecuritySeverityMapping(t *testing.T) {
	cases := []struct {
		level, confidence, want string
	}{
		{"fail", "high", "9.3"},
		{"fail", "medium", "8.1"},
		{"fail", "", "8.1"},
		{"fail", "low", "6.5"},
		{"warn", "high", "6.1"},
		{"warn", "medium", "5.0"},
		{"warn", "", "5.0"},
		{"warn", "low", "3.1"},
	}
	for _, tc := range cases {
		doc := parseSARIF(t, renderSARIF(t, findingsReport("security", codeguard.Finding{
			RuleID: "security.weak-hash", Level: tc.level, Confidence: tc.confidence, Message: "md5", Path: "a.go", Line: 2,
		})))
		if got := ruleByID(t, doc, "security.weak-hash").Properties.SecuritySeverity; got != tc.want {
			t.Errorf("level=%s confidence=%q: security-severity = %q, want %q", tc.level, tc.confidence, got, tc.want)
		}
	}

	// The rule-level score takes the strongest finding of the run.
	doc := parseSARIF(t, renderSARIF(t, findingsReport("security",
		codeguard.Finding{RuleID: "security.weak-hash", Level: "warn", Confidence: "low", Message: "a", Path: "a.go", Line: 1},
		codeguard.Finding{RuleID: "security.weak-hash", Level: "fail", Confidence: "high", Message: "b", Path: "b.go", Line: 1},
	)))
	rule := ruleByID(t, doc, "security.weak-hash")
	if rule.Properties.SecuritySeverity != "9.3" || rule.Properties.Precision != "high" {
		t.Errorf("rule-level score/precision = %q/%q, want 9.3/high", rule.Properties.SecuritySeverity, rule.Properties.Precision)
	}

	// CI workflow rules with a CWE are classified as security too.
	doc = parseSARIF(t, renderSARIF(t, findingsReport("ci", codeguard.Finding{
		RuleID: "ci.workflow-script-injection", Level: "fail", Confidence: "high", Message: "x", Path: ".github/workflows/a.yml", Line: 4,
	})))
	ci := ruleByID(t, doc, "ci.workflow-script-injection")
	if !containsTag(ci.Properties.Tags, "security") || !containsTag(ci.Properties.Tags, "ci") || ci.Properties.SecuritySeverity == "" {
		t.Errorf("ci.workflow-script-injection should be tagged ci+security and scored: %+v", ci.Properties)
	}
}

// A real Go taint scan must produce a code flow that starts at the source
// line, walks the intermediate variables, and ends at the sink line.
func TestSARIFGoTaintFindingHasCodeFlow(t *testing.T) {
	raw := renderSARIF(t, scanGoTaintFixture(t, t.TempDir()))
	doc := parseSARIF(t, raw)
	result := resultByRule(t, doc, "security.taint.go")
	if len(result.CodeFlows) != 1 || len(result.CodeFlows[0].ThreadFlows) != 1 {
		t.Fatalf("expected one codeFlow with one threadFlow:\n%s", raw)
	}
	steps := result.CodeFlows[0].ThreadFlows[0].Locations
	if len(steps) != 4 {
		t.Fatalf("expected source, 2 intermediate steps, sink; got %d:\n%s", len(steps), raw)
	}
	source, sink := steps[0], steps[len(steps)-1]
	if !containsTag(source.Kinds, "source") || source.Location.PhysicalLocation == nil || source.Location.PhysicalLocation.Region == nil ||
		source.Location.PhysicalLocation.Region.StartLine != 9 || !strings.Contains(source.Location.Message.Text, "os.Getenv") {
		t.Errorf("source step wrong: %+v", source)
	}
	if !containsTag(sink.Kinds, "sink") || sink.Location.PhysicalLocation == nil || sink.Location.PhysicalLocation.Region == nil ||
		sink.Location.PhysicalLocation.Region.StartLine != 11 || !strings.Contains(sink.Location.Message.Text, "exec.Command") {
		t.Errorf("sink step wrong: %+v", sink)
	}
	for _, step := range steps[1 : len(steps)-1] {
		if step.Location.Message == nil || step.Location.PhysicalLocation == nil || step.Location.PhysicalLocation.ArtifactLocation.URI == "" {
			t.Errorf("intermediate step should keep its file and message: %+v", step)
		}
	}
	if !strings.Contains(steps[1].Location.Message.Text, "userCmd") || !strings.Contains(steps[2].Location.Message.Text, "alias") {
		t.Errorf("intermediate steps should follow userCmd -> alias: %+v", steps[1:3])
	}
	if errs := loadSARIFSchema(t).validateSARIF(t, raw); len(errs) > 0 {
		t.Errorf("Go taint SARIF violates the schema:\n%s", strings.Join(errs, "\n"))
	}
}

// The TypeScript engine records a file:line per step, so each step gets its
// own location, including steps in other modules.
func TestSARIFTypeScriptTaintStepsCarryTheirOwnLocations(t *testing.T) {
	doc := parseSARIF(t, renderSARIF(t, findingsReport("security", codeguard.Finding{
		RuleID: "security.typescript.taint-flow", Level: "warn", Path: "src/db.ts", Line: 20, Column: 1,
		Message: "tainted data flow: req.query (src/app.ts:12) → call lookup (src/app.ts:14) → query sink (src/db.ts:20)",
	})))
	steps := resultByRule(t, doc, "security.typescript.taint-flow").CodeFlows[0].ThreadFlows[0].Locations
	want := []struct {
		uri  string
		line int
	}{{"src/app.ts", 12}, {"src/app.ts", 14}, {"src/db.ts", 20}}
	if len(steps) != len(want) {
		t.Fatalf("got %d steps, want %d", len(steps), len(want))
	}
	for i, step := range steps {
		loc := step.Location.PhysicalLocation
		if loc == nil || loc.Region == nil || loc.ArtifactLocation.URI != want[i].uri || loc.Region.StartLine != want[i].line {
			t.Errorf("step %d location = %+v, want %s:%d", i, loc, want[i].uri, want[i].line)
		}
	}
}

// GitHub deduplicates alerts across uploads by partialFingerprints, so two
// scans of the same tree must emit identical, non-empty fingerprints.
func TestSARIFFingerprintsAreStableAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	first := parseSARIF(t, renderSARIF(t, scanGoTaintFixture(t, dir)))
	second := parseSARIF(t, renderSARIF(t, scanGoTaintFixture(t, dir)))
	fingerprints := func(doc sarifDoc) []map[string]string {
		var out []map[string]string
		for _, result := range doc.Runs[0].Results {
			out = append(out, result.PartialFingerprints)
		}
		return out
	}
	a, b := fingerprints(first), fingerprints(second)
	if len(a) == 0 {
		t.Fatal("expected results from the taint fixture")
	}
	for _, fp := range a {
		if fp["codeguardContext/v1"] == "" || fp["codeguardLegacy/v1"] == "" {
			t.Errorf("result missing a partial fingerprint: %v", fp)
		}
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("partialFingerprints changed between runs:\n%v\n%v", a, b)
	}
}

// A report exercising every result shape codeguard emits must validate against
// the official SARIF 2.1.0 schema.
func TestSARIFOutputValidatesAgainstSchema(t *testing.T) {
	report := findingsReport("security",
		codeguard.Finding{RuleID: "security.taint.go", Level: "fail", Confidence: "high", Path: "main.go", Line: 5, Column: 3, Fingerprint: "fp1", ContextFingerprint: "ctx1",
			Message: "tainted data from os.Getenv (line 3) reaches exec.Command (line 5) via cmd -> exec.Command"},
		codeguard.Finding{RuleID: "security.typescript.taint-flow", Level: "warn", Path: "src/db.ts", Line: 20,
			Message: "tainted data flow: req.query (src/app.ts:12) → query sink (src/db.ts:20)"},
		codeguard.Finding{RuleID: "supply_chain.missing-lockfile", Level: "fail", Message: "no lockfile", Path: "package.json"},
		codeguard.Finding{RuleID: "custom.pack.rule", Level: "warn", Message: "custom rule without catalog metadata"},
	)
	report.Profile = "default"
	report.Sections[0].Diagnostics = []core.Diagnostic{
		{ID: "scan.file-limit", Level: "warn", Kind: "analysis", Message: "walk stopped", Path: "vendor/", Operational: true},
	}
	schema := loadSARIFSchema(t)
	raw := renderSARIF(t, report)
	if errs := schema.validateSARIF(t, raw); len(errs) > 0 {
		t.Fatalf("SARIF violates the 2.1.0 schema:\n%s\n\n%s", strings.Join(errs, "\n"), raw)
	}

	// Negative control: the validator must reject a document that breaks the
	// schema, or the assertion above would pass vacuously.
	broken := strings.Replace(string(raw), `"level": "error"`, `"level": "fatal"`, 1)
	broken = strings.Replace(broken, `"startLine": 5`, `"startLine": 0`, 1)
	if errs := schema.validateSARIF(t, []byte(broken)); len(errs) < 2 {
		t.Fatalf("validator accepted an invalid level and startLine; errors: %v", errs)
	}
}

// A crashed section reports scan.section.panic; the SARIF invocation must then
// say the analysis did not complete, while a clean report says it did.
func TestSARIFExecutionSuccessfulIsFalseOnSectionPanic(t *testing.T) {
	crashed, panicked := runnerchecks.SafeRun("security", "Security", func() core.SectionResult {
		var findings []core.Finding
		_ = findings[3]
		return core.SectionResult{}
	})
	if !panicked {
		t.Fatal("SafeRun should have recovered the panic")
	}
	healthy := codeguard.SectionResult{ID: "quality", Name: "Code Quality", Status: codeguard.StatusPass}

	raw := renderSARIF(t, codeguard.Report{Sections: []codeguard.SectionResult{healthy, crashed}})
	doc := parseSARIF(t, raw)
	if len(doc.Runs[0].Invocations) != 1 || doc.Runs[0].Invocations[0].ExecutionSuccessful {
		t.Fatalf("executionSuccessful should be false after a section panic:\n%s", raw)
	}
	if got := resultByRule(t, doc, runnerchecks.SectionPanicDiagnosticID).Level; got != "error" {
		t.Errorf("panic diagnostic level = %q, want error", got)
	}
	if errs := loadSARIFSchema(t).validateSARIF(t, raw); len(errs) > 0 {
		t.Errorf("panic SARIF violates the schema:\n%s", strings.Join(errs, "\n"))
	}

	clean := parseSARIF(t, renderSARIF(t, codeguard.Report{Sections: []codeguard.SectionResult{healthy}}))
	if !clean.Runs[0].Invocations[0].ExecutionSuccessful {
		t.Error("executionSuccessful should be true for a clean report")
	}
	driver := clean.Runs[0].Tool.Driver
	if driver.Name != "codeguard" || driver.Version == "" || driver.InformationURI == "" {
		t.Errorf("driver metadata incomplete: %+v", driver)
	}
}

// helpUri anchors must resolve: every catalog rule needs its
// <a id="rule-..."></a> anchor in docs/checks.md.
func TestDocsChecksHasAnAnchorForEveryCatalogRule(t *testing.T) {
	docs, err := os.ReadFile(filepath.Join("..", "..", "docs", "checks.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(docs)
	for id := range rulespkg.Catalog() {
		if !strings.Contains(text, `<a id="`+rulespkg.DocsAnchor(id)+`"></a>`) {
			t.Errorf("docs/checks.md has no anchor for %s; add `<a id=%q></a>` to the rule index", id, rulespkg.DocsAnchor(id))
		}
	}
}
