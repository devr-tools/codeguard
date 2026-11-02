package support_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// TestCyclomaticComplexityMatchesGocyclo pins the Go metric to gocyclo's
// definition of McCabe complexity: 1, plus one per if/for/range, per
// non-default case or comm clause, and per && or || operator. Function
// literals count toward the enclosing function.
func TestCyclomaticComplexityMatchesGocyclo(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"straight line", `x := 1; _ = x`, 1},
		{"if and else", `if a { return } else { return }`, 2},
		{"short-circuit operators", `if a && b || c { return }`, 4},
		{"operators outside conditions", `ok := a && b; _ = ok`, 2},
		{"for and range", `for i := 0; i < n; i++ {}; for range xs {}`, 3},
		{"switch default is not a decision", `switch x { case 1: case 2, 3: default: }`, 3},
		{"type switch", `switch x.(type) { case int: case string: default: }`, 3},
		{"select default is not a decision", `select { case <-ch: case ch2 <- 1: default: }`, 3},
		{"function literal counts", `f := func() { if a || b {} }; f()`, 3},
		{"bitwise operators do not count", `_ = a & b | c`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := runnersupport.CyclomaticComplexity(parseFuncBody(t, tc.body)); got != tc.want {
				t.Fatalf("CyclomaticComplexity(%q) = %d, want %d", tc.body, got, tc.want)
			}
		})
	}
	if got := runnersupport.CyclomaticComplexity(nil); got != 0 {
		t.Fatalf("CyclomaticComplexity(nil) = %d, want 0", got)
	}
}

// TestLegacyCyclomaticComplexityMatchesPreGocycloCount pins the legacy mode
// (the 1.x default) to the Go count codeguard reported through v1.10.1: 1,
// plus one per if/for/range and per case or comm clause, default included,
// with && and || not counted.
func TestLegacyCyclomaticComplexityMatchesPreGocycloCount(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"straight line", `x := 1; _ = x`, 1},
		{"if and else", `if a { return } else { return }`, 2},
		{"short-circuit operators do not count", `if a && b || c { return }`, 2},
		{"operators outside conditions", `ok := a && b; _ = ok`, 1},
		{"for and range", `for i := 0; i < n; i++ {}; for range xs {}`, 3},
		{"switch default counts", `switch x { case 1: case 2, 3: default: }`, 4},
		{"type switch default counts", `switch x.(type) { case int: case string: default: }`, 4},
		{"select default counts", `select { case <-ch: case ch2 <- 1: default: }`, 4},
		{"function literal counts", `f := func() { if a || b {} }; f()`, 2},
		{"bitwise operators do not count", `_ = a & b | c`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := runnersupport.LegacyCyclomaticComplexity(parseFuncBody(t, tc.body)); got != tc.want {
				t.Fatalf("LegacyCyclomaticComplexity(%q) = %d, want %d", tc.body, got, tc.want)
			}
		})
	}
	if got := runnersupport.LegacyCyclomaticComplexity(nil); got != 0 {
		t.Fatalf("LegacyCyclomaticComplexity(nil) = %d, want 0", got)
	}
}

// TestCyclomaticComplexityForSelectsMode checks the mode-to-function wiring:
// unset and legacy select the legacy count, gocyclo (any case) selects the
// gocyclo count.
func TestCyclomaticComplexityForSelectsMode(t *testing.T) {
	body := parseFuncBody(t, `if a && b || c {}; switch x { case 1: default: }`)
	const legacy, gocyclo = 4, 5
	for _, tc := range []struct {
		mode string
		want int
	}{
		{"", legacy},
		{"legacy", legacy},
		{" Legacy ", legacy},
		{"gocyclo", gocyclo},
		{"GOCYCLO", gocyclo},
	} {
		if got := runnersupport.CyclomaticComplexityFor(tc.mode)(body); got != tc.want {
			t.Errorf("CyclomaticComplexityFor(%q) = %d, want %d", tc.mode, got, tc.want)
		}
	}
}

func parseFuncBody(t *testing.T, body string) *ast.BlockStmt {
	t.Helper()
	src := "package p\nfunc f(a, b, c bool, n int, x any, xs []int, ch, ch2 chan int) {\n" + body + "\n}\n"
	file, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	if err != nil {
		t.Fatalf("parse %q: %v", body, err)
	}
	return file.Decls[0].(*ast.FuncDecl).Body
}
