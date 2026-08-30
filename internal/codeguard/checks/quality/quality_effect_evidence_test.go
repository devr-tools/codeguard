package quality

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
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

func TestFunctionMutationAnalysisRetainsUnresolvedGoRootWithoutFindingEvidence(t *testing.T) {
	fn := parseGoPrecisionFunctionForTest(t, `package sample
func ReadValue() int {
	mystery.Value = 1
	return mystery.Value
}`)
	analysis := functionMutationAnalysis(fn, "go")
	assertOnlyUnresolvedMutation(t, analysis, "go", "mystery", "assignment")
}

func TestFunctionMutationAnalysisRetainsUnknownFactoryResultWithoutFindingEvidence(t *testing.T) {
	fn := parseGoPrecisionFunctionForTest(t, `package sample
func ReadValue() int {
	item := opaqueFactory()
	item.Field = 1
	return item.Field
}`)
	for idx := range fn.Assignments {
		if fn.Assignments[idx].Name == "item" {
			fn.Assignments[idx].Expr = "opaqueFactory()"
		}
	}
	fn.ProvenGlobals = map[string]struct{}{"item": {}}
	analysis := functionMutationAnalysis(fn, "go")
	assertOnlyUnresolvedMutation(t, analysis, "go", "item", "assignment")
}

func TestFunctionMutationAnalysisRetainsUnresolvedCPPRootWithoutFindingEvidence(t *testing.T) {
	parsed := support.ParseCLike(`int readValue() {
  mystery->value = 1;
  return mystery->value;
}`, support.CLikeCPP)
	functions := parsed.AllFunctions()
	if len(functions) != 1 {
		t.Fatalf("functions = %d, want 1", len(functions))
	}
	analysis := functionMutationAnalysis(parsedPrecisionFunction(functions[0]), "c++")
	assertOnlyUnresolvedMutation(t, analysis, "c++", "mystery", "assignment")
}

func parseGoPrecisionFunctionForTest(t *testing.T, source string) precisionFunction {
	t.Helper()
	data := []byte(source)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", data, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range file.Decls {
		if declaration, ok := item.(*ast.FuncDecl); ok {
			return goPrecisionFunction(fset, declaration, data)
		}
	}
	t.Fatal("function declaration missing")
	return precisionFunction{}
}

func assertOnlyUnresolvedMutation(t *testing.T, analysis mutationAnalysis, language string, symbol string, operation string) {
	t.Helper()
	if len(analysis.Mutations) != 0 {
		t.Fatalf("mutation findings evidence = %#v, want none", analysis.Mutations)
	}
	if len(analysis.Unresolved) != 1 {
		t.Fatalf("unresolved evidence = %#v, want one record", analysis.Unresolved)
	}
	record := analysis.Unresolved[0]
	if record.Language != language || record.Symbol != symbol || record.Operation != operation || record.Line <= 0 || record.Reason == "" {
		t.Fatalf("unresolved record = %#v, want language=%q symbol=%q operation=%q positive line and reason", record, language, symbol, operation)
	}
}

func TestUnresolvedMutationDiagnosticsAggregateSeparatelyByLanguage(t *testing.T) {
	diagnostics := unresolvedMutationDiagnostics([]unresolvedMutationEvidence{
		{Language: "go", Line: 3, Operation: "assignment", Symbol: "first", Reason: "unknown"},
		{Language: "c++", Line: 8, Operation: "assignment", Symbol: "second", Reason: "unknown"},
		{Language: "go", Line: 13, Operation: "call", Symbol: "third", Reason: "unknown"},
	})
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics = %#v, want one per language", diagnostics)
	}
	wantCounts := map[string]string{"c++": "1", "go": "2"}
	for _, diagnostic := range diagnostics {
		if diagnostic.ID != "quality.structural-unresolved-symbols" || diagnostic.Level != "info" || diagnostic.Operational {
			t.Fatalf("diagnostic = %#v, want non-operational informational structural diagnostic", diagnostic)
		}
		language := diagnostic.Metadata["language"]
		if diagnostic.Metadata["count"] != wantCounts[language] {
			t.Fatalf("diagnostic metadata = %#v, want count %q for language %q", diagnostic.Metadata, wantCounts[language], language)
		}
		delete(wantCounts, language)
	}
	if len(wantCounts) != 0 {
		t.Fatalf("missing language diagnostics: %#v", wantCounts)
	}
}

func TestLanguageQualityAnalysisCollectsUnresolvedEvidenceWhenFindingScanIsCached(t *testing.T) {
	target := core.TargetConfig{Name: "repo", Path: t.TempDir(), Language: "go"}
	env := support.Context{
		Config: core.Config{Targets: []core.TargetConfig{target}},
		// A cache hit returns findings without invoking the file evaluator.
		ScanTargetFiles: func(core.TargetConfig, string, func(string) bool, func(string, []byte) []core.Finding) []core.Finding {
			return nil
		},
		VisitTargetFiles: func(_ core.TargetConfig, include func(string) bool, visit func(string, []byte)) {
			if include("fixture.go") {
				visit("fixture.go", []byte("package sample\nfunc ReadValue() int {\n mystery.Value = 1\n return mystery.Value\n}\n"))
			}
		},
	}

	analysis := languageQualityAnalysis(context.Background(), env, target)
	if len(analysis.findings) != 0 {
		t.Fatalf("findings = %#v, want none", analysis.findings)
	}
	if len(analysis.unresolved) != 1 || analysis.unresolved[0].Language != "go" || analysis.unresolved[0].Symbol != "mystery" {
		t.Fatalf("unresolved = %#v, want cached-scan-independent Go record", analysis.unresolved)
	}
}
