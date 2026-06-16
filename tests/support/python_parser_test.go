package support_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

const trickyPython = `import os, sys as system
from subprocess import run as launch, Popen

DOC = """
def not_a_function(x):
    pass
"""

def outer(
    first,
    second: int = 2,
    *args,
    **kwargs,
):
    text = 'def fake(y):'
    cmd = f"echo {first}"
    def inner(value):
        return value + 1
    return inner(first)

async def fetch(url: str) -> str:
    payload = "# not a comment"
    return url
`

func TestParsePythonStructure(t *testing.T) {
	file := support.ParsePython(trickyPython)

	if len(file.Functions) != 2 {
		names := make([]string, 0)
		for _, fn := range file.Functions {
			names = append(names, fn.Name)
		}
		t.Fatalf("expected 2 top-level functions, got %d (%v)", len(file.Functions), names)
	}
	outer := file.FunctionByName("outer")
	if outer == nil {
		t.Fatal("outer not found")
	}
	if len(outer.Params) != 4 {
		t.Fatalf("outer params = %+v, want 4", outer.Params)
	}
	if outer.Params[1].Name != "second" || outer.Params[1].Type != "int" {
		t.Fatalf("param annotation lost: %+v", outer.Params[1])
	}
	if len(outer.Nested) != 1 || outer.Nested[0].Name != "inner" {
		t.Fatalf("nested functions = %+v", outer.Nested)
	}
	if outer.StartLine != 9 {
		t.Fatalf("outer start line = %d, want 9", outer.StartLine)
	}
	if file.FunctionByName("not_a_function") != nil {
		t.Fatal("function inside triple-quoted string must not parse")
	}
	if file.FunctionByName("fake") != nil {
		t.Fatal("function inside string literal must not parse")
	}
}

func TestParsePythonSymbolsAndImports(t *testing.T) {
	file := support.ParsePython(trickyPython)

	outer := file.FunctionByName("outer")
	if kind, ok := outer.Lookup("first"); !ok || kind != support.SymbolParam {
		t.Fatalf("first = (%v,%v), want param", kind, ok)
	}
	if kind, ok := outer.Lookup("cmd"); !ok || kind != support.SymbolLocal {
		t.Fatalf("cmd = (%v,%v), want local", kind, ok)
	}
	if _, ok := outer.Lookup("missing"); ok {
		t.Fatal("missing must not resolve")
	}

	wantImports := map[string]string{"os": "os", "system": "sys", "launch": "subprocess", "Popen": "subprocess"}
	for alias, module := range wantImports {
		if !hasImport(file.Imports, module, alias) {
			t.Fatalf("missing import %s as %s in %+v", module, alias, file.Imports)
		}
	}
}

func TestParsePythonMaskKeepsFStringExpressions(t *testing.T) {
	file := support.ParsePython(trickyPython)
	outer := file.FunctionByName("outer")

	var cmd *support.ParsedAssignment
	for idx := range outer.Assignments {
		if outer.Assignments[idx].Name == "cmd" {
			cmd = &outer.Assignments[idx]
		}
	}
	if cmd == nil {
		t.Fatalf("cmd assignment not found in %+v", outer.Assignments)
	}
	if !strings.Contains(cmd.Expr, "{first}") {
		t.Fatalf("f-string interpolation lost: %q", cmd.Expr)
	}
	if strings.Contains(cmd.Expr, "echo") {
		t.Fatalf("string contents must be masked: %q", cmd.Expr)
	}
}

func TestParsePythonMultilineCallsAndStatements(t *testing.T) {
	source := strings.Join([]string{
		"def runner(target):",
		"    result = launch(",
		"        target,",
		"        check=True,",
		"    )",
		"    return result",
		"",
	}, "\n")
	file := support.ParsePython(source)
	runner := file.FunctionByName("runner")
	if runner == nil {
		t.Fatal("runner not found")
	}
	if len(runner.Statements) != 2 {
		t.Fatalf("statements = %d, want 2 logical statements", len(runner.Statements))
	}
	if len(runner.Calls) == 0 || runner.Calls[0].Callee != "launch" {
		t.Fatalf("calls = %+v", runner.Calls)
	}
	if len(runner.Calls[0].Args) != 2 {
		t.Fatalf("launch args = %+v", runner.Calls[0].Args)
	}
	if runner.EndLine != 6 {
		t.Fatalf("runner end line = %d, want 6", runner.EndLine)
	}
}

func hasImport(imports []support.ParsedImport, module string, alias string) bool {
	for _, imp := range imports {
		if imp.Module == module && imp.Alias == alias {
			return true
		}
	}
	return false
}
