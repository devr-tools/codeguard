package quality

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestFunctionMutationEvidenceTracksSameLineEscape(t *testing.T) {
	source := []byte(`package sample
type State struct{ Value int }; var sharedState *State
func CurrentState() *State { state := &State{}; sharedState = state; state.Value = 1; return state }`)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "escape.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	var declaration *ast.FuncDecl
	for _, item := range file.Decls {
		if fn, ok := item.(*ast.FuncDecl); ok && fn.Name.Name == "CurrentState" {
			declaration = fn
		}
	}
	if declaration == nil {
		t.Fatal("CurrentState declaration missing")
	}
	fn := goPrecisionFunction(fset, declaration, source)
	evidence, ok := firstReportableMutationEvidence(fn)
	if !ok || evidence.Target != targetEscaped || evidence.Origin != originShared {
		t.Fatalf("evidence = %#v, ok=%v; assignments=%#v statements=%#v", evidence, ok, fn.Assignments, fn.Statements)
	}
}
