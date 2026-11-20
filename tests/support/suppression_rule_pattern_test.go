package support_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/rules"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func TestClassifyRulePattern(t *testing.T) {
	cases := []struct {
		pattern string
		kind    rules.RulePatternKind
		prefix  string
	}{
		{"*", rules.RulePatternAll, ""},
		{"security.taint.go", rules.RulePatternExact, ""},
		{"security.*", rules.RulePatternPrefix, "security."},
		{"security.taint.*", rules.RulePatternPrefix, "security.taint."},
		{" security.* ", rules.RulePatternPrefix, "security."},
		{"security*", rules.RulePatternInvalid, ""},
		{"*.go", rules.RulePatternInvalid, ""},
		{"security.*.go", rules.RulePatternInvalid, ""},
		{"security.**", rules.RulePatternInvalid, ""},
		{".*", rules.RulePatternInvalid, ""},
		{"**", rules.RulePatternInvalid, ""},
		{"security..*", rules.RulePatternInvalid, ""},
	}
	for _, tc := range cases {
		kind, prefix := rules.ClassifyRulePattern(tc.pattern)
		if kind != tc.kind || prefix != tc.prefix {
			t.Errorf("ClassifyRulePattern(%q) = (%v, %q), want (%v, %q)", tc.pattern, kind, prefix, tc.kind, tc.prefix)
		}
	}
	if !rules.MatchRulePattern("security.*", "security.taint.go") {
		t.Error("security.* must match security.taint.go")
	}
	if rules.MatchRulePattern("security.*", "securityx.rule") {
		t.Error("security.* must not match securityx.rule (prefix is dotted)")
	}
	if rules.MatchRulePattern("security.taint.*", "security.shell-execution") {
		t.Error("security.taint.* must not match security.shell-execution")
	}
	if rules.MatchRulePattern("security*", "security.taint.go") {
		t.Error("an invalid pattern must match nothing")
	}
}

func writeSuppressionPatternFile(t *testing.T, content string) runnersupport.Context {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "app.ts"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sc := runnersupport.Context{
		Cfg:   core.Config{Targets: []core.TargetConfig{{Name: "repo", Path: dir}}},
		Today: inlineSuppressionToday,
	}
	return sc
}

func TestInlineSuppressionPrefixPatterns(t *testing.T) {
	sc := writeSuppressionPatternFile(t, strings.Join([]string{
		"// codeguard:ignore security.* -- reviewed", // 1: covers 1 and 2
		"const a = 1;", // 2
		"// codeguard:ignore security.taint.* until 2000-01-01 -- expired", // 3
		"const b = 2;", // 4
		"// codeguard:ignore security.taint.* until 2999-01-01 -- live", // 5
		"const c = 3;", // 6
		"",
	}, "\n"))
	cases := []struct {
		rule string
		line int
		want bool
	}{
		{"security.taint.go", 2, true},
		{"security.shell-execution", 2, true},
		{"quality.max-function-lines", 2, false},
		{"security.taint.go", 4, false}, // expired prefix directive keeps until semantics
		{"security.taint.go", 6, true},
		{"security.shell-execution", 6, false},
	}
	for _, tc := range cases {
		got := runnersupport.MatchSuppression(sc, core.Finding{RuleID: tc.rule, Path: "src/app.ts", Line: tc.line})
		if (got != nil) != tc.want {
			t.Errorf("%s at line %d: suppression = %+v, want match=%t", tc.rule, tc.line, got, tc.want)
		}
	}
}

func TestInlineSuppressionInvalidPatternWarns(t *testing.T) {
	sc := writeSuppressionPatternFile(t, strings.Join([]string{
		"// codeguard:ignore security* -- typo", // 1
		"const a = 1;",                          // 2
		"const b = 2;",                          // 3
		"",
	}, "\n"))
	finding := core.Finding{RuleID: "security.taint.go", Path: "src/app.ts", Line: 2, Level: "fail", Fingerprint: "fp"}
	if got := runnersupport.MatchSuppression(sc, finding); got != nil {
		t.Fatalf("invalid pattern suppressed the finding: %+v", got)
	}
	section := runnersupport.FinalizeSection(sc, "security", "Security", []core.Finding{finding})
	if len(section.Findings) != 1 {
		t.Fatalf("findings = %+v, want the finding still reported", section.Findings)
	}
	var found *core.Diagnostic
	for i := range section.Diagnostics {
		if section.Diagnostics[i].ID == runnersupport.InvalidRulePatternDiagnosticID {
			found = &section.Diagnostics[i]
		}
	}
	if found == nil {
		t.Fatalf("diagnostics = %+v, want %s", section.Diagnostics, runnersupport.InvalidRulePatternDiagnosticID)
	}
	if found.Level != "warn" || found.Path != "src/app.ts" || found.Metadata["line"] != "1" || !strings.Contains(found.Message, "security*") {
		t.Fatalf("diagnostic = %+v, want a warn naming src/app.ts:1 and the pattern", *found)
	}

	// A finding the directive could not have covered does not raise it.
	far := core.Finding{RuleID: "security.taint.go", Path: "src/app.ts", Line: 3, Level: "fail", Fingerprint: "fp3"}
	section = runnersupport.FinalizeSection(sc, "security", "Security", []core.Finding{far})
	for _, diagnostic := range section.Diagnostics {
		if diagnostic.ID == runnersupport.InvalidRulePatternDiagnosticID {
			t.Fatalf("diagnostic raised for a finding the directive does not cover: %+v", diagnostic)
		}
	}
}

func TestWaiverAuditStatusForRulePatterns(t *testing.T) {
	catalog := map[string]core.RuleMetadata{"security.taint.go": {ID: "security.taint.go"}}
	waivers := []core.WaiverConfig{{Rule: "security.*"}, {Rule: "nosuch.*"}, {Rule: "security*"}}
	entries := runnersupport.NewWaiverAuditCollector(waivers).Snapshot(catalog, inlineSuppressionToday)
	want := []string{core.WaiverAuditStatusUnused, core.WaiverAuditStatusUnknownRule, core.WaiverAuditStatusUnknownRule}
	for i, entry := range entries {
		if entry.Status != want[i] {
			t.Errorf("waiver %q status = %q, want %q", entry.Rule, entry.Status, want[i])
		}
	}
}

func TestWaiverPrefixPatterns(t *testing.T) {
	sc := runnersupport.Context{Cfg: core.Config{Waivers: []core.WaiverConfig{
		{Rule: "security.taint.*", Path: "src/**"},
		{Rule: "quality*", Path: "src/**"},
	}}, Today: inlineSuppressionToday}
	if got := runnersupport.MatchingWaivers(sc, core.Finding{RuleID: "security.taint.go", Path: "src/a.go"}); len(got) != 1 || got[0].Index != 0 {
		t.Fatalf("waivers = %+v, want the prefix waiver", got)
	}
	if got := runnersupport.MatchingWaivers(sc, core.Finding{RuleID: "security.shell-execution", Path: "src/a.go"}); len(got) != 0 {
		t.Fatalf("waivers = %+v, want none outside the prefix", got)
	}
	if got := runnersupport.MatchingWaivers(sc, core.Finding{RuleID: "quality.max-function-lines", Path: "src/a.go"}); len(got) != 0 {
		t.Fatalf("waivers = %+v, an invalid pattern must match nothing", got)
	}
	diagnostics := runnersupport.InvalidWaiverPatternDiagnostics(sc.Cfg.Waivers)
	if len(diagnostics) != 1 || diagnostics[0].Level != "warn" || !strings.Contains(diagnostics[0].Message, "quality*") {
		t.Fatalf("diagnostics = %+v, want one warn naming quality*", diagnostics)
	}
}
