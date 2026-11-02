package checks_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// TestQualityMutableGlobalStateModuleScopeEquivalence pins the module-scope
// gate quality.mutable-global-state applies to TS/JS files. The gate is a
// deliberately simple line-local brace counter: every '{' and '}' before a
// line's first "//" counts (including ones inside strings, template literals,
// regex literals and block comments), the depth is clamped at zero after each
// line, and indented lines never count as module scope. Each case ends with a
// top-level probe assignment; want is the line the rule reports (the first
// module-scope mutable assignment it sees) or 0 when the gate hides the probe.
// The expected values were produced by the original per-line rescanning
// implementation, so this test guards the single-pass rewrite bit for bit.
func TestQualityMutableGlobalStateModuleScopeEquivalence(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   int
	}{
		{"nested_balanced.ts", "export function outer() {\n  if (a) {\n    for (;;) { }\n  }\n}\nprobe = 1\n", 6},
		{"nested_unclosed.ts", "function f() {\n  if (x) {\n  }\nprobe = 1\n", 0},
		{"string_open_brace.ts", "const s = \"{\";\nprobe = 1\n", 0},
		{"string_close_braces_clamp.ts", "const s = \"}}\";\nprobe = 1\n", 2},
		{"string_comment_marker.ts", "const url = \"http://x{\";\nprobe = 1\n", 2},
		{"template_balanced.ts", "const t = `${a} and ${b}`;\nprobe = 1\n", 2},
		{"template_unbalanced.ts", "const t = `${a`;\nprobe = 1\n", 0},
		{"template_multiline.js", "const t = `line {\nprobe = 1\n}`;\nprobe = 2\n", 4},
		{"template_nested_interp.ts", "const t = `${fn({ a: `${b}` })}`;\nprobe = 1\n", 2},
		{"block_comment_open.ts", "/* { */\nprobe = 1\n", 0},
		{"block_comment_multiline.ts", "/*\n * {\n * }\n */\nprobe = 1\n", 5},
		{"line_comment_brace.ts", "// {\nprobe = 1\n", 2},
		{"trailing_comment_brace.ts", "function f() { // }\n}\nprobe = 1\n", 3},
		{"indented_probe.ts", "  probe = 1\n\tprobe = 2\nprobe = 3\n", 3},
		{"crlf.ts", "function f() {\r\n  x = 1\r\n}\r\nprobe = 1\r\n", 4},
		{"lone_cr_eof.ts", "probe = 1\r", 1},
		{"negative_clamp.js", "}}\n{\n}\nprobe = 1\n", 4},
		{"else_chain.ts", "if (a) {\n} else {\n  y = 2\n}\nprobe = 1\n", 5},
		{"unterminated_string.js", "const s = \"abc{\nprobe = 1\n", 0},
		{"regex_literal.js", "const re = /[{]/;\nprobe = 1\n", 0},
		{"jsx_double_brace.tsx", "export const C = () => <div style={{ a: 1 }}>{x}</div>;\nprobe = 1\n", 2},
		{"jsx_multiline.jsx", "export function C() {\n  return (\n    <div>{items.map((i) => { return i; })}</div>\n  );\n}\nprobe = 1\n", 6},
		{"probe_opens_brace.ts", "probe = {\n  a: 1,\n};\nother = 2\n", 1},
		{"let_inside_function.ts", "export function f() {\n  let count = 0;\n  count = 1;\n}\nexport let total = 0;\n", 5},
		{"object_literal_close_same_line.ts", "const cfg = { a: { b: 1 } };\nprobe = 1\n", 2},
		{"generated_many_functions.js", generatedModuleScopeSource(300), 300*4 + 1},
	}

	dir := t.TempDir()
	for _, tc := range cases {
		writeFile(t, filepath.Join(dir, "src", tc.name), tc.source)
	}
	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	got := map[string]int{}
	for _, finding := range findingsForRule(report, "quality.mutable-global-state") {
		name := filepath.Base(filepath.FromSlash(finding.Path))
		if prev, ok := got[name]; ok {
			t.Fatalf("%s: expected at most one finding per file, got lines %d and %d", name, prev, finding.Line)
		}
		got[name] = finding.Line
	}
	for _, tc := range cases {
		if got[tc.name] != tc.want {
			t.Errorf("%s: mutable-global-state line = %d, want %d", tc.name, got[tc.name], tc.want)
		}
		delete(got, tc.name)
	}
	for name, line := range got {
		t.Errorf("unexpected mutable-global-state finding in %s at line %d", name, line)
	}
}

func generatedModuleScopeSource(functions int) string {
	var b strings.Builder
	for i := 0; i < functions; i++ {
		fmt.Fprintf(&b, "export function f%d(a) {\n  const s = \"{}\" + `${a}`;\n  return { s };\n}\n", i)
	}
	b.WriteString("probe = 1\n")
	return b.String()
}
