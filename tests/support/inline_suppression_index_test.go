package support_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// inlineSuppressionFixtures pins every directive shape the per-path index must
// reproduce. Line numbers are named in the cases below.
var inlineSuppressionFixtures = map[string]string{
	"src/native.ts": strings.Join([]string{
		"const a = 1;",                       // 1
		"// codeguard:ignore quality.rule-a", // 2: covers 2 and 3
		"const b = 2;",                       // 3
		"const c = 3;",                       // 4: not covered
		"const d = 4; // codeguard:ignore *", // 5: wildcard covers 5 and 6
		"const e = 5;",                       // 6
		"// codeguard:ignore quality.rule-b until 2000-01-01", // 7: expired
		"const f = 6;", // 8
		"// codeguard:ignore quality.rule-c until 2999-01-01", // 9: live
		"const g = 7;", // 10
		"const h = 8; // codeguard:ignore quality.rule-d codeguard:ignore quality.rule-e", // 11: two on one line
		"",
	}, "\n"),
	"src/plain.ts": "export function plain() {\n  return 1;\n}\n",
	"cmd/run.go": strings.Join([]string{
		"package main", // 1
		"",             // 2
		"import \"os/exec\"",
		"",                        // 4
		"func run(name string) {", // 5
		"\t_ = exec.Command(name).Run() //nolint:gosec // validated tool path",        // 6: exact line 6
		"\t_ = exec.Command(name).Run() //nolint:gosec",                               // 7: unjustified
		"\t//nolint:gosec // standalone covers the next statement",                    // 8: exact line 9
		"\t_ = exec.Command(name).Run()",                                              // 9
		"\t// #nosec G204 -- operator-supplied binary",                                // 10: exact line 11
		"\t_ = exec.Command(name).Run()",                                              // 11
		"\t_ = exec.Command(name).Run() // #nosec G304 -- not the subprocess rule",    // 12: wrong gosec rule
		"\t//nolint:gosec // gosec record first",                                      // 13: exact line 14, earlier in source
		"\t_ = exec.Command(name).Run() // codeguard:ignore security.shell-execution", // 14: native second
		"\t// codeguard:ignore security.shell-execution",                              // 15: native first
		"\t_ = exec.Command(name).Run() //nolint:gosec // gosec record second",        // 16: gosec second
		"\t// codeguard:ignore security.shell-execution until 2000-01-01",             // 17: expired native
		"\t_ = exec.Command(name).Run() //nolint:gosec // falls through to gosec",     // 18
		"}",
		"",
	}, "\n"),
	"notes.txt": "exec.Command(name) //nolint:gosec // looks justified\n",
}

type inlineSuppressionCase struct {
	name      string
	path      string
	rule      string
	line      int
	wantMatch bool
	wantLabel string
}

var inlineSuppressionCases = []inlineSuppressionCase{
	{"directive line", "src/native.ts", "quality.rule-a", 2, true, ""},
	{"next line", "src/native.ts", "quality.rule-a", 3, true, ""},
	{"two lines below", "src/native.ts", "quality.rule-a", 4, false, ""},
	{"line above", "src/native.ts", "quality.rule-a", 1, false, ""},
	{"other rule", "src/native.ts", "quality.other", 3, false, ""},
	{"file-level finding", "src/native.ts", "quality.rule-a", 0, true, ""},
	{"wildcard same line", "src/native.ts", "quality.anything", 5, true, ""},
	{"wildcard next line", "src/native.ts", "quality.anything", 6, true, ""},
	{"wildcard file-level", "src/native.ts", "quality.anything", 0, true, ""},
	{"expired", "src/native.ts", "quality.rule-b", 8, false, ""},
	{"file-level falls past expired to wildcard", "src/native.ts", "quality.rule-b", 0, true, ""},
	{"live until", "src/native.ts", "quality.rule-c", 10, true, ""},
	{"first of two on a line", "src/native.ts", "quality.rule-d", 11, true, ""},
	{"second of two on a line", "src/native.ts", "quality.rule-e", 12, true, ""},
	{"no directives", "src/plain.ts", "quality.rule-a", 2, false, ""},
	{"no directives file-level", "src/plain.ts", "quality.rule-a", 0, false, ""},
	{"missing file", "src/missing.ts", "quality.rule-a", 1, false, ""},
	{"directory", "src", "quality.rule-a", 1, false, ""},
	{"empty path", "", "quality.rule-a", 1, false, ""},
	{"nolint trailing", "cmd/run.go", "security.shell-execution", 6, true, runnersupport.SuppressionMatchGosecJustification},
	{"nolint trailing is exact", "cmd/run.go", "security.shell-execution", 7, false, ""},
	{"nolint unjustified", "cmd/run.go", "security.shell-execution", 7, false, ""},
	{"nolint other rule", "cmd/run.go", "security.taint.go", 6, false, ""},
	{"standalone nolint own line", "cmd/run.go", "security.shell-execution", 8, false, ""},
	{"standalone nolint next line", "cmd/run.go", "security.shell-execution", 9, true, runnersupport.SuppressionMatchGosecJustification},
	{"nosec next line", "cmd/run.go", "security.shell-execution", 11, true, runnersupport.SuppressionMatchGosecJustification},
	{"nosec wrong gosec rule", "cmd/run.go", "security.shell-execution", 12, false, ""},
	{"file-level skips exact gosec", "cmd/run.go", "security.shell-execution", 0, true, ""},
	{"earlier gosec wins", "cmd/run.go", "security.shell-execution", 14, true, runnersupport.SuppressionMatchGosecJustification},
	{"earlier native wins", "cmd/run.go", "security.shell-execution", 16, true, ""},
	{"expired native falls through", "cmd/run.go", "security.shell-execution", 18, true, runnersupport.SuppressionMatchGosecJustification},
	{"gosec outside go source", "notes.txt", "security.shell-execution", 1, false, ""},
}

func writeInlineSuppressionFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range inlineSuppressionFixtures {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

func assertInlineSuppressionCases(t *testing.T, sc runnersupport.Context) {
	t.Helper()
	for _, tc := range inlineSuppressionCases {
		finding := core.Finding{RuleID: tc.rule, Path: tc.path, Line: tc.line}
		got := runnersupport.MatchSuppression(sc, finding)
		if !tc.wantMatch {
			if got != nil {
				t.Errorf("%s: suppression = %+v, want none", tc.name, got)
			}
			continue
		}
		if got == nil {
			t.Errorf("%s: no suppression, want inline", tc.name)
			continue
		}
		if got.Kind != "inline" || got.Match != tc.wantLabel {
			t.Errorf("%s: suppression = %+v, want inline with match %q", tc.name, got, tc.wantLabel)
		}
	}
}

var inlineSuppressionToday = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

// TestInlineSuppressionIndexWithoutCorpus covers the uncached path a bare
// Context takes: every lookup reads and parses the file afresh.
func TestInlineSuppressionIndexWithoutCorpus(t *testing.T) {
	dir := writeInlineSuppressionFixtures(t)
	sc := runnersupport.Context{
		Cfg:   core.Config{Targets: []core.TargetConfig{{Name: "repo", Path: dir}}},
		Today: inlineSuppressionToday,
	}
	assertInlineSuppressionCases(t, sc)
}

// TestInlineSuppressionIndexWithCorpus covers the per-scan memoized index: the
// same cases must hold on the first lookup, on repeat lookups served from the
// cache, and under concurrent lookups (run with -race).
func TestInlineSuppressionIndexWithCorpus(t *testing.T) {
	dir := writeInlineSuppressionFixtures(t)
	cfg := core.Config{Targets: []core.TargetConfig{{Name: "repo", Path: dir}}}
	sc, err := runnersupport.NewContext(context.Background(), cfg, core.ScanOptions{Mode: core.ScanModeFull})
	if err != nil {
		t.Fatalf("new context: %v", err)
	}
	sc.Today = inlineSuppressionToday
	assertInlineSuppressionCases(t, sc)
	assertInlineSuppressionCases(t, sc)

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assertInlineSuppressionCases(t, sc)
		}()
	}
	wg.Wait()
}

// TestInlineSuppressionIndexResolvesFirstExistingTarget pins target
// resolution: the file under the first target where the path exists is the one
// whose directives apply, even when a later target holds a directive.
func TestInlineSuppressionIndexResolvesFirstExistingTarget(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	if err := os.WriteFile(filepath.Join(first, "app.ts"), []byte("const a = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "app.ts"), []byte("// codeguard:ignore *\nconst a = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "only.ts"), []byte("// codeguard:ignore *\nconst a = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := core.Config{Targets: []core.TargetConfig{{Name: "one", Path: first}, {Name: "two", Path: second}}}
	sc, err := runnersupport.NewContext(context.Background(), cfg, core.ScanOptions{Mode: core.ScanModeFull})
	if err != nil {
		t.Fatalf("new context: %v", err)
	}
	if got := runnersupport.MatchSuppression(sc, core.Finding{RuleID: "x.y", Path: "app.ts", Line: 2}); got != nil {
		t.Fatalf("app.ts resolved past the first target: %+v", got)
	}
	if got := runnersupport.MatchSuppression(sc, core.Finding{RuleID: "x.y", Path: "only.ts", Line: 2}); got == nil || got.Kind != "inline" {
		t.Fatalf("only.ts suppression = %+v, want inline from the second target", got)
	}
}
