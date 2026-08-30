package quality

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestGoScopeOriginResolutionIgnoresSyntaxPackagesAndShadows(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "default import",
			source: `package sample
import "fmt"
func ReadValue() int { fmt.Set(); return 1 }`,
		},
		{
			name: "aliased import",
			source: `package sample
import storage "example.com/storage"
func ReadValue() int { storage.Update(); return 1 }`,
		},
		{
			name: "control flow keywords",
			source: `package sample
type State struct{ Value int }
func ReadValue() int {
	for index := 0; index < 1; index++ {
		if true { local := &State{}; local.Value = index }
	}
	return 1
}`,
		},
		{
			name: "local shadows global",
			source: `package sample
type State struct{ Value int }
var state *State
func ReadValue() int { state := &State{}; state.Value = 1; return state.Value }`,
		},
		{
			name: "nested block shadows global",
			source: `package sample
type State struct{ Value int }
var state *State
func ReadValue() int {
	if true { state := &State{}; state.Value = 1 }
	return 1
}`,
		},
		{
			name: "initializer scope shadows global",
			source: `package sample
type State struct{ Value int }
var state *State
func ReadValue() int {
	if state := (&State{}); state != nil { state.Value = 1 }
	return 1
}`,
		},
		{
			name: "closure local shadows global",
			source: `package sample
type State struct{ Value int }
var state *State
func ReadValue() int {
	func() { state := &State{}; state.Value = 1 }()
	return 1
}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := parseGoMutationAnalysisForTest(t, test.source, "ReadValue")
			if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
				t.Fatalf("analysis = %#v, want no mutation or unresolved evidence", analysis)
			}
		})
	}
}

func TestGoScopeOriginResolutionDoesNotTreatImportedMutatorAsHiddenSideEffect(t *testing.T) {
	source := `package sample
import storage "example.com/storage"
func LookupValue() int { storage.Update(); return 1 }`
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "fixture.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	index := newGoPackageIndex()
	index.addFile("fixture.go", fset, parsed)
	fn := goPrecisionFunctionByNameForTest(t, fset, parsed, []byte(source), "LookupValue")
	fn.GoFile = "fixture.go"
	fn.GoPackage = index.packageFor("fixture.go", parsed.Name.Name)
	if hiddenSideEffect("fixture.go", fn) {
		t.Fatal("imported package call was classified as a hidden side effect")
	}
}

func TestGoClosureOriginResolutionUsesCapturedDeclaration(t *testing.T) {
	analysis := parseGoMutationAnalysisForTest(t, `package sample
type State struct{ Value int }
func ReadValue(input *State) int {
	mutate := func() { input.Value = 1 }
	mutate()
	return input.Value
}`, "ReadValue")
	assertGoMutation(t, analysis, targetArgument, originCaller)
}

func TestGoClosureOriginResolutionUsesInvocationArgumentOwnership(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantTarget string
	}{
		{
			name: "local pointer argument remains local",
			source: `package sample
type State struct{ Value int }
func ReadValue() int {
	local := &State{}
	func(state *State) { state.Value = 1 }(local)
	return local.Value
}`,
		},
		{
			name: "caller pointer argument remains caller owned",
			source: `package sample
type State struct{ Value int }
func ReadValue(input *State) int {
	func(state *State) { state.Value = 1 }(input)
	return input.Value
}`,
			wantTarget: targetArgument,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := parseGoMutationAnalysisForTest(t, test.source, "ReadValue")
			if test.wantTarget == "" {
				if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
					t.Fatalf("analysis = %#v, want closure-local mutation", analysis)
				}
				return
			}
			assertGoMutation(t, analysis, test.wantTarget, originCaller)
		})
	}
}

func TestGoScopeOriginResolutionRetainsUnresolvedWithoutMutation(t *testing.T) {
	analysis := parseGoMutationAnalysisForTest(t, `package sample
func ReadValue() int { mystery.Value = 1; return mystery.Value }`, "ReadValue")
	assertOnlyUnresolvedMutation(t, analysis, "go", "mystery", "assignment")
}

func TestGoReferenceShapeOriginResolution(t *testing.T) {
	declarations := `package sample
type Node struct{ Name string }
type Payload struct {
	Name string
	Tags []string
	Meta map[string]string
	Node *Node
	Any any
}
`
	tests := []struct {
		name       string
		body       string
		wantTarget string
		wantOrigin string
	}{
		{name: "value field reassignment is local", body: `func ReadValue(input Payload) string { input.Name = "local"; return input.Name }`},
		{name: "map field index reaches caller content", body: `func ReadValue(input Payload) string { input.Meta["status"] = "ready"; return input.Meta["status"] }`, wantTarget: targetArgument, wantOrigin: originCaller},
		{name: "slice field index reaches caller content", body: `func ReadValue(input Payload) string { input.Tags[0] = "ready"; return input.Tags[0] }`, wantTarget: targetArgument, wantOrigin: originCaller},
		{name: "pointer field reaches caller pointee", body: `func ReadValue(input Payload) string { input.Node.Name = "ready"; return input.Node.Name }`, wantTarget: targetArgument, wantOrigin: originCaller},
		{name: "asserted interface map reaches caller content", body: `func ReadValue(input Payload) string { input.Any.(map[string]string)["status"] = "ready"; return "ready" }`, wantTarget: targetArgument, wantOrigin: originCaller},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := parseGoMutationAnalysisForTest(t, declarations+test.body, "ReadValue")
			if test.wantTarget == "" {
				if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
					t.Fatalf("analysis = %#v, want local-only mutation", analysis)
				}
				return
			}
			assertGoMutation(t, analysis, test.wantTarget, test.wantOrigin)
		})
	}
}

func TestGoReferenceFieldReassignmentUpdatesContentOwnership(t *testing.T) {
	declarations := `package sample
type Node struct{ Name string }
type Payload struct {
	Tags []string
	Meta map[string]string
	Node *Node
	Any any
}
`
	tests := []struct {
		name string
		body string
	}{
		{name: "map field", body: `input.Meta = make(map[string]string); input.Meta["status"] = "local"`},
		{name: "slice field", body: `input.Tags = make([]string, 1); input.Tags[0] = "local"`},
		{name: "pointer field", body: `input.Node = &Node{}; input.Node.Name = "local"`},
		{name: "interface field", body: `input.Any = map[string]string{}; input.Any.(map[string]string)["status"] = "local"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := parseGoMutationAnalysisForTest(t, declarations+`func ReadValue(input Payload) string { `+test.body+`; return "ok" }`, "ReadValue")
			if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
				t.Fatalf("analysis = %#v, want reassigned local field content to remain local", analysis)
			}
		})
	}
}

func TestGoReferenceFieldReassignmentCanRetainCallerOwnership(t *testing.T) {
	analysis := parseGoMutationAnalysisForTest(t, `package sample
type Payload struct{ Meta map[string]string }
func ReadValue(input Payload, replacement map[string]string) string {
	input.Meta = replacement
	input.Meta["status"] = "ready"
	return input.Meta["status"]
}`, "ReadValue")
	assertGoMutation(t, analysis, targetArgument, originCaller)
}

func TestGoReferenceFieldReassignmentDoesNotLeakAcrossBranches(t *testing.T) {
	analysis := parseGoMutationAnalysisForTest(t, `package sample
type Payload struct{ Meta map[string]string }
func ReadValue(input Payload, flag bool) string {
	if flag { input.Meta = make(map[string]string) } else { input.Meta["status"] = "ready" }
	return "ok"
}`, "ReadValue")
	assertGoMutation(t, analysis, targetArgument, originCaller)
}

func TestGoAliasOriginResolutionFollowsReferenceShapesOnly(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantTarget string
	}{
		{
			name: "pointer aliases",
			source: `package sample
type Node struct{ Name string }
func ReadValue(input *Node) string { a := input; b := a; b.Name = "ready"; return b.Name }`,
			wantTarget: targetArgument,
		},
		{
			name: "map aliases",
			source: `package sample
func ReadValue(input map[string]string) string { a := input; b := a; b["status"] = "ready"; return b["status"] }`,
			wantTarget: targetArgument,
		},
		{
			name: "slice aliases",
			source: `package sample
func ReadValue(input []string) string { a := input; b := a; b[0] = "ready"; return b[0] }`,
			wantTarget: targetArgument,
		},
		{
			name: "range pointer aliases",
			source: `package sample
type Node struct{ Name string }
func ReadValue(input []*Node) string { for _, node := range input { node.Name = "ready" }; return input[0].Name }`,
			wantTarget: targetArgument,
		},
		{
			name: "value aliases remain local",
			source: `package sample
type Node struct{ Name string }
func ReadValue(input Node) string { a := input; b := a; b.Name = "local"; return b.Name }`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := parseGoMutationAnalysisForTest(t, test.source, "ReadValue")
			if test.wantTarget == "" {
				if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
					t.Fatalf("analysis = %#v, want local-only mutation", analysis)
				}
				return
			}
			assertGoMutation(t, analysis, test.wantTarget, originCaller)
		})
	}
}

func TestGoRangeAssignmentUpdatesExistingAlias(t *testing.T) {
	analysis := parseGoMutationAnalysisForTest(t, `package sample
type Node struct{ Value int }
func ReadValue(input []*Node) int {
	var node *Node
	for _, node = range input { node.Value = 1 }
	return input[0].Value
}`, "ReadValue")
	assertGoMutation(t, analysis, targetArgument, originCaller)
}

func TestGoRangeAssignmentDoesNotLeakAliasAfterOptionalLoop(t *testing.T) {
	analysis := parseGoMutationAnalysisForTest(t, `package sample
type Node struct{ Value int }
func ReadValue(input []*Node) int {
	var node *Node
	for _, node = range input {}
	node.Value = 1
	return node.Value
}`, "ReadValue")
	if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
		t.Fatalf("analysis = %#v, want no alias proof after a possibly empty range", analysis)
	}
}

func TestGoClosureCapturesDeclarationVisibleAtDefinition(t *testing.T) {
	analysis := parseGoMutationAnalysisForTest(t, `package sample
type Node struct{ Value int }
var shared *Node
func ReadValue() int {
	mutate := func() { shared.Value = 1 }
	shared := &Node{}
	mutate()
	return shared.Value
}`, "ReadValue")
	assertGoMutation(t, analysis, targetGlobal, originShared)
}

func TestGoShadowedBuiltinsUseNearestDeclaration(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "append",
			source: `package sample
func ReadValue(input []string) int { append := func([]string, string) {}; append(input, "x"); return len(input) }`,
		},
		{
			name: "copy",
			source: `package sample
func ReadValue(input []string) int { copy := func([]string, []string) {}; copy(input, nil); return len(input) }`,
		},
		{
			name: "delete",
			source: `package sample
func ReadValue(input map[string]string) int { delete := func(map[string]string, string) {}; delete(input, "x"); return len(input) }`,
		},
		{
			name: "clear",
			source: `package sample
func ReadValue(input map[string]string) int { clear := func(map[string]string) {}; clear(input); return len(input) }`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := parseGoMutationAnalysisForTest(t, test.source, "ReadValue")
			if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
				t.Fatalf("analysis = %#v, want shadowed callable behavior only", analysis)
			}
		})
	}
}

func TestGoBranchEscapeRequiresEveryReachableArm(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantTarget string
	}{
		{
			name: "escape and mutation in exclusive arms",
			body: `if flag { shared = local } else { local.Value = 1 }`,
		},
		{
			name: "optional escape before later mutation",
			body: `if flag { shared = local }; local.Value = 1`,
		},
		{
			name: "switch arms are exclusive",
			body: `switch flag { case true: shared = local; default: local.Value = 1 }`,
		},
		{
			name:       "escape in both arms before later mutation",
			body:       `if flag { shared = local } else { shared = local }; local.Value = 1`,
			wantTarget: targetEscaped,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := parseGoMutationAnalysisForTest(t, `package sample
type Node struct{ Value int }
var shared *Node
func ReadValue(flag bool) int { local := &Node{}; `+test.body+`; return local.Value }`, "ReadValue")
			if test.wantTarget == "" {
				if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
					t.Fatalf("analysis = %#v, want no cross-branch escape proof", analysis)
				}
				return
			}
			assertGoMutation(t, analysis, test.wantTarget, originShared)
		})
	}
}

func TestGoReferenceOriginResolutionMarksOnlyPostEscapeMutation(t *testing.T) {
	analysis := parseGoMutationAnalysisForTest(t, `package sample
type Node struct{ Value int }
var shared *Node
func ReadValue() int {
	local := &Node{}
	local.Value = 1
	shared = local
	local.Value = 2
	return local.Value
}`, "ReadValue")
	if len(analysis.Mutations) != 1 {
		t.Fatalf("mutations = %#v, want exactly one post-escape mutation", analysis.Mutations)
	}
	assertGoMutation(t, analysis, targetEscaped, originShared)
}

func parseGoMutationAnalysisForTest(t *testing.T, source string, functionName string) mutationAnalysis {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "fixture.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	index := newGoPackageIndex()
	index.addFile("fixture.go", fset, parsed)
	fn := goPrecisionFunctionByNameForTest(t, fset, parsed, []byte(source), functionName)
	fn.GoPackage = index.packageFor("fixture.go", parsed.Name.Name)
	return goFunctionMutationEvidence(fn)
}

func goPrecisionFunctionByNameForTest(t *testing.T, fset *token.FileSet, parsed *ast.File, source []byte, functionName string) precisionFunction {
	t.Helper()
	for _, declaration := range parsed.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if ok && fn.Name.Name == functionName {
			return goPrecisionFunction(fset, fn, source)
		}
	}
	t.Fatalf("function %q declaration missing", functionName)
	return precisionFunction{}
}

func assertGoMutation(t *testing.T, analysis mutationAnalysis, target string, origin string) {
	t.Helper()
	if len(analysis.Unresolved) != 0 {
		t.Fatalf("unresolved = %#v, want none", analysis.Unresolved)
	}
	for _, mutation := range analysis.Mutations {
		if mutation.Target == target && mutation.Origin == origin && mutation.Effect == "shared_state" {
			return
		}
	}
	t.Fatalf("mutations = %#v, want target=%q origin=%q shared-state evidence", analysis.Mutations, target, origin)
}
