package quality

import (
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func TestCppOriginDeclarationsRecordTypesScopesAndAliases(t *testing.T) {
	parsed := support.ParseCLike(`
struct Token { int value; };
Token sharedToken{};

struct DbRow {
  Token memberToken{};
  DbRow() : memberToken{} { memberToken.value = 0; }
};

template <typename T>
int DbRow::integer(T input) {
  Token plain;
  Token braced{};
  auto constructed = Token{};
  constexpr int limit = 3;
  T templated{};
  auto builder = make_builder();
  Token& alias = sharedToken;
  if (limit > 0) {
    Token sharedToken{};
    sharedToken.value++;
  }
  builder.set_value(input);
  return memberToken.value;
}
`, support.CLikeCPP)
	fn := cppFunctionForTest(t, parsed, "DbRow::integer")

	want := map[string]struct {
		kind, typ, shape, alias string
	}{
		"input":       {kind: "parameter", typ: "T"},
		"plain":       {kind: "local", typ: "Token"},
		"braced":      {kind: "local", typ: "Token"},
		"constructed": {kind: "local", typ: "auto"},
		"limit":       {kind: "local", typ: "constexpr int"},
		"templated":   {kind: "local", typ: "T"},
		"builder":     {kind: "local", typ: "auto"},
		"alias":       {kind: "local", typ: "Token&", shape: "reference", alias: "sharedToken"},
		"sharedToken": {kind: "global", typ: "Token"},
		"memberToken": {kind: "member", typ: "Token"},
	}
	for name, expectation := range want {
		declaration, ok := cppDeclarationForTest(fn.Declarations, name, expectation.kind)
		if !ok {
			t.Fatalf("missing %s declaration %q: %#v", expectation.kind, name, fn.Declarations)
		}
		if declaration.Type != expectation.typ || declaration.ReferenceShape != expectation.shape || declaration.AliasSource != expectation.alias {
			t.Fatalf("declaration %q = %#v, want type=%q shape=%q alias=%q", name, declaration, expectation.typ, expectation.shape, expectation.alias)
		}
		if declaration.Line <= 0 || declaration.ScopeStart <= 0 || declaration.ScopeEnd < declaration.ScopeStart {
			t.Fatalf("declaration %q has invalid lexical span: %#v", name, declaration)
		}
	}
	if fn.QualifiedOwner != "DbRow" {
		t.Fatalf("qualified owner = %q, want DbRow", fn.QualifiedOwner)
	}
	for _, keyword := range []string{"auto", "constexpr", "template", "typename", "return"} {
		if _, ok := cppDeclarationForTest(fn.Declarations, keyword, ""); ok {
			t.Fatalf("keyword %q became a declaration: %#v", keyword, fn.Declarations)
		}
	}

	constructor := cppFunctionForTest(t, parsed, "DbRow")
	if constructor.EndLine < constructor.StartLine || len(constructor.Statements) == 0 {
		t.Fatalf("constructor member initializer terminated function discovery: %#v", constructor)
	}
}

func TestCppOriginValueParametersAndLocalsRemainLocal(t *testing.T) {
	cases := map[string]string{
		"value parameter":      `int current(Token input) { input.value++; return input.value; }`,
		"plain local":          `int current() { Token item; item.value++; return item.value; }`,
		"braced local":         `int current() { Token item{}; item.value++; return item.value; }`,
		"constructed auto":     `int current() { auto item = Token{}; item.value++; return item.value; }`,
		"moved local":          `int current() { Token item{}; auto moved = std::move(item); moved.value++; return moved.value; }`,
		"typed builder result": `int current() { DbRow row = DbRow::integer(1); row.set_value(2); return row.value; }`,
		"lambda init capture":  `int current() { Token item{}; auto work = [copy = std::move(item)]() mutable { copy.value++; }; work(); return 1; }`,
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			analysis := cppAnalysisForTest(t, source, "current")
			if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
				t.Fatalf("analysis = %#v, want local-only operation", analysis)
			}
		})
	}
}

func TestCppOriginUnknownAutoCallResultsNeverUseCalleeNames(t *testing.T) {
	for name, initializer := range map[string]string{
		"make prefix":           "make_widget()",
		"create prefix":         "create_widget()",
		"build method":          "Widget::build()",
		"integer method":        "DbRow::integer(1)",
		"capitalized qualifier": "Widget::from_row()",
		"templated call":        "std::make_unique<Token>()",
	} {
		t.Run(name, func(t *testing.T) {
			analysis := cppAnalysisForTest(t, `int current() {
  auto item = `+initializer+`;
  item.update();
  return 1;
}`, "current")
			assertCppUnresolvedContains(t, analysis, "item", "call")
		})
	}
}

func TestCppOriginReferencesPointersAndAliasesRetainCallerOwnership(t *testing.T) {
	cases := map[string]string{
		"reference":             `int current(Token& input) { input.value++; return input.value; }`,
		"pointer":               `int current(Token* input) { input->value++; return input->value; }`,
		"reference alias chain": `int current(Token& input) { Token& first = input; auto& second = first; second.value++; return second.value; }`,
		"pointer alias chain":   `int current(Token* input) { Token* first = input; auto* second = first; second->value++; return second->value; }`,
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			analysis := cppAnalysisForTest(t, source, "current")
			assertCppMutationForTest(t, analysis, targetArgument, originCaller)
		})
	}
}

func TestCppOriginAutoPointerAliasesRetainCallerOwnership(t *testing.T) {
	cases := map[string]string{
		"address of reference": `int current(Token& input) { auto alias = &input; alias->value++; return alias->value; }`,
		"pointer copy":         `int current(Token* inputPtr) { auto alias = inputPtr; alias->value++; return alias->value; }`,
		"multi hop pointer copy": `int current(Token* inputPtr) {
  auto first = inputPtr;
  auto second = first;
  second->value++;
  return second->value;
}`,
		"explicit pointer then auto": `int current(Token& input) {
  Token* first = &input;
  auto second = first;
  second->value++;
  return second->value;
}`,
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			analysis := cppAnalysisForTest(t, source, "current")
			assertCppMutationForTest(t, analysis, targetArgument, originCaller)
		})
	}
}

func TestCppLambdaCaptureOwnershipDistinguishesValueAndReference(t *testing.T) {
	value := cppAnalysisForTest(t, `int current(Token& input) {
  auto work = [input]() mutable { input.value++; };
  work();
  return input.value;
}`, "current")
	if len(value.Mutations) != 0 || len(value.Unresolved) != 0 {
		t.Fatalf("value capture analysis = %#v, want local copy", value)
	}

	reference := cppAnalysisForTest(t, `int current(Token& input) {
  auto work = [&input]() { input.value++; };
  work();
  return input.value;
}`, "current")
	assertCppMutationForTest(t, reference, targetArgument, originCaller)
}

func TestCppLambdaDefaultAndNestedCaptureOwnership(t *testing.T) {
	t.Run("default value copies reference parameter", func(t *testing.T) {
		analysis := cppAnalysisForTest(t, `int current(Token& input) {
  auto work = [=]() mutable { input.value++; };
  work();
  return input.value;
}`, "current")
		if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
			t.Fatalf("analysis = %#v, want value-captured local copy", analysis)
		}
	})

	t.Run("default reference retains parameter", func(t *testing.T) {
		analysis := cppAnalysisForTest(t, `int current(Token& input) {
  auto work = [&]() { input.value++; };
  work();
  return input.value;
}`, "current")
		assertCppMutationForTest(t, analysis, targetArgument, originCaller)
	})

	t.Run("nested default value copies outer reference capture", func(t *testing.T) {
		analysis := cppAnalysisForTest(t, `int current(Token& input) {
  auto outer = [&input]() {
    auto inner = [=]() mutable { input.value++; };
    inner();
  };
  outer();
  return input.value;
}`, "current")
		if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
			t.Fatalf("analysis = %#v, want nested value-captured local copy", analysis)
		}
	})
}

func TestCppLambdaLocalDeclarationOutranksDefaultCapture(t *testing.T) {
	analysis := cppAnalysisForTest(t, `struct Counter {
  Token state;
  int current() {
    auto work = [=]() {
      Token* p = &state;
      p->value++;
    };
    work();
    return state.value;
  }
};`, "current")
	assertCppMutationForTest(t, analysis, targetReceiver, originCaller)
	if len(analysis.Unresolved) != 0 {
		t.Fatalf("unresolved = %#v, want lambda local alias resolved to member", analysis.Unresolved)
	}
}

func TestCppOriginUnknownCallResultRemainsUnresolved(t *testing.T) {
	analysis := cppAnalysisForTest(t, `int current() {
  auto item = opaque_factory();
  item.update();
  return 1;
}`, "current")
	assertOnlyUnresolvedMutation(t, analysis, "c++", "item", "call")
}

func TestCppOriginReadOnlyMemberCallsDoNotBecomeMutations(t *testing.T) {
	cases := map[string]string{
		"saved member size":     `int current(const Profile& profile) { return profile.savedPlateIds.size(); }`,
		"added member size":     `int current(const Snapshot& snapshot) { return snapshot.addedCellIds.size(); }`,
		"saved member contains": `bool current(const Lookups& lookups) { return lookups.savedPlateIds.contains("id"); }`,
		"saved member empty":    `bool current(const Filter& filter) { return filter.savedPlateIds.empty(); }`,
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			analysis := cppAnalysisForTest(t, source, "current")
			if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
				t.Fatalf("analysis = %#v, want read-only member call", analysis)
			}
		})
	}
}

func TestCppOriginObservableEffectsUseTerminalMethodWords(t *testing.T) {
	event := cppAnalysisForTest(t, `int current(Dispatcher& dispatcher) { dispatcher.tryEnqueue(); return 1; }`, "current")
	if len(event.Mutations) != 1 || event.Mutations[0].Effect != "event" || event.Mutations[0].Target != targetArgument {
		t.Fatalf("event analysis = %#v, want caller-owned event mutation", event)
	}
	read := cppAnalysisForTest(t, `int current(const Upload& upload) { return upload.expiresAt.Format(); }`, "current")
	if len(read.Mutations) != 0 || len(read.Unresolved) != 0 {
		t.Fatalf("read analysis = %#v, want receiver field names excluded from effect classification", read)
	}
}

func TestCppConstructorMutationIsExplicitByLanguageSyntax(t *testing.T) {
	parsed := support.ParseCLike(`
struct Registry {
  Counter requestCount;
  Registry() { requestCount = makeCounter(); }
};
`, support.CLikeCPP)
	fn := parsedPrecisionFunction(cppFunctionForTest(t, parsed, "Registry"))
	assertCppMutationForTest(t, cppFunctionMutationEvidence(fn), targetReceiver, originCaller)
	if evidence, ok := hiddenMutationEvidence("registry.cpp", fn); ok {
		t.Fatalf("constructor produced hidden mutation evidence: %#v", evidence)
	}
}

func TestCppQualifiedCommandNameExposesMutation(t *testing.T) {
	parsed := support.ParseCLike(`
struct TraceSpan { State node; void setStatus(); };
void TraceSpan::setStatus() { node.value++; }
`, support.CLikeCPP)
	fn := parsedPrecisionFunction(cppFunctionForTest(t, parsed, "TraceSpan::setStatus"))
	assertCppMutationForTest(t, cppFunctionMutationEvidence(fn), targetReceiver, originCaller)
	if evidence, ok := hiddenMutationEvidence("tracing.cpp", fn); ok {
		t.Fatalf("qualified command name produced hidden mutation evidence: %#v", evidence)
	}
}

func TestCppOriginResolvesReceiverGlobalShadowingAndEscapes(t *testing.T) {
	t.Run("receiver member", func(t *testing.T) {
		analysis := cppAnalysisForTest(t, `struct Counter {
  Token state;
  int current() { state.value++; return state.value; }
};`, "current")
		assertCppMutationForTest(t, analysis, targetReceiver, originCaller)
	})

	t.Run("qualified receiver member", func(t *testing.T) {
		analysis := cppAnalysisForTest(t, `struct Counter { Token state; int current(); };
int Counter::current() { state.value++; return state.value; }`, "Counter::current")
		assertCppMutationForTest(t, analysis, targetReceiver, originCaller)
	})

	t.Run("proven global", func(t *testing.T) {
		analysis := cppAnalysisForTest(t, `Token shared;
int current() { shared.value++; return shared.value; }`, "current")
		assertCppMutationForTest(t, analysis, targetGlobal, originShared)
	})

	t.Run("local shadows global", func(t *testing.T) {
		analysis := cppAnalysisForTest(t, `Token shared;
int current() { Token shared{}; shared.value++; return shared.value; }`, "current")
		if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
			t.Fatalf("analysis = %#v, want shadowing local", analysis)
		}
	})

	t.Run("local shadows member", func(t *testing.T) {
		analysis := cppAnalysisForTest(t, `struct Counter {
  Token state;
  int current() { Token state{}; state.value++; return state.value; }
};`, "current")
		if len(analysis.Mutations) != 0 || len(analysis.Unresolved) != 0 {
			t.Fatalf("analysis = %#v, want shadowing local", analysis)
		}
	})

	t.Run("escaped local", func(t *testing.T) {
		analysis := cppAnalysisForTest(t, `Token* shared;
int current() { Token item{}; shared = &item; item.value++; return item.value; }`, "current")
		assertCppMutationForTest(t, analysis, targetEscaped, originShared)
	})
}

func TestCppOriginUnknownMutationRemainsUnresolved(t *testing.T) {
	analysis := cppAnalysisForTest(t, `int current() { mystery.update(); return 1; }`, "current")
	assertOnlyUnresolvedMutation(t, analysis, "c++", "mystery", "call")
}

func TestCppOriginUnknownBareMutationsRemainUnresolved(t *testing.T) {
	for name, operation := range map[string]string{
		"assignment": "mystery = value;",
		"increment":  "mystery++;",
	} {
		t.Run(name, func(t *testing.T) {
			analysis := cppAnalysisForTest(t, `int current() { `+operation+` return 1; }`, "current")
			assertOnlyUnresolvedMutation(t, analysis, "c++", "mystery", "assignment")
		})
	}
}

func TestCppOriginUnknownBarePrefixMutationsRemainUnresolved(t *testing.T) {
	for name, operation := range map[string]string{
		"increment": "++mystery;",
		"decrement": "--mystery;",
	} {
		t.Run(name, func(t *testing.T) {
			analysis := cppAnalysisForTest(t, `int current() { `+operation+` return 1; }`, "current")
			assertOnlyUnresolvedMutation(t, analysis, "c++", "mystery", "assignment")
		})
	}
}

func TestCppOriginNamespaceQualifiedReceiverResolvesMember(t *testing.T) {
	parsed := support.ParseCLike(`namespace N {
struct Counter {
  Token state;
  int current();
};
}
int N::Counter::current() { state.value++; return state.value; }`, support.CLikeCPP)
	fn := cppFunctionForTest(t, parsed, "N::Counter::current")
	if fn.QualifiedOwner != "N::Counter" {
		t.Fatalf("qualified owner = %q, want N::Counter", fn.QualifiedOwner)
	}
	analysis := cppFunctionMutationEvidence(parsedPrecisionFunction(fn))
	assertCppMutationForTest(t, analysis, targetReceiver, originCaller)
	if len(analysis.Unresolved) != 0 {
		t.Fatalf("unresolved = %#v, want namespace/type tokens excluded", analysis.Unresolved)
	}
}

func TestCppOriginNestedNamespaceQualifiedReceiverResolvesMember(t *testing.T) {
	parsed := support.ParseCLike(`namespace N {
namespace M {
struct Counter {
  Token state;
  int current();
};
}
}
int N::M::Counter::current() { state.value++; return state.value; }`, support.CLikeCPP)
	fn := cppFunctionForTest(t, parsed, "N::M::Counter::current")
	if fn.QualifiedOwner != "N::M::Counter" {
		t.Fatalf("qualified owner = %q, want N::M::Counter", fn.QualifiedOwner)
	}
	analysis := cppFunctionMutationEvidence(parsedPrecisionFunction(fn))
	assertCppMutationForTest(t, analysis, targetReceiver, originCaller)
	if len(analysis.Unresolved) != 0 {
		t.Fatalf("unresolved = %#v, want nested namespace/type tokens excluded", analysis.Unresolved)
	}
}

func TestCppCommandQueryUsesResolvedPersistenceOwnership(t *testing.T) {
	parsed := support.ParseCLike(`int current(Repository& repo) {
  repo.save();
  return 1;
}`, support.CLikeCPP)
	fn := parsedPrecisionFunction(cppFunctionForTest(t, parsed, "current"))
	evidence, ok := commandQueryEvidence("current.cpp", fn)
	if !ok || evidence.Target != targetArgument || evidence.Origin != originCaller || evidence.Effect != "persistence" {
		t.Fatalf("evidence = %#v, ok=%v, want argument/caller-owned persistence", evidence, ok)
	}
}

func cppAnalysisForTest(t *testing.T, source string, name string) mutationAnalysis {
	t.Helper()
	parsed := support.ParseCLike(source, support.CLikeCPP)
	return cppFunctionMutationEvidence(parsedPrecisionFunction(cppFunctionForTest(t, parsed, name)))
}

func cppFunctionForTest(t *testing.T, parsed *support.ParsedFile, name string) *support.ParsedFunction {
	t.Helper()
	for _, fn := range parsed.AllFunctions() {
		if fn.Name == name || strings.HasSuffix(fn.Name, "::"+name) {
			return fn
		}
	}
	t.Fatalf("function %q missing: %#v", name, parsed.AllFunctions())
	return nil
}

func cppDeclarationForTest(declarations []support.ParsedDeclaration, name string, kind string) (support.ParsedDeclaration, bool) {
	for _, declaration := range declarations {
		if declaration.Name == name && (kind == "" || declaration.Kind == kind) {
			return declaration, true
		}
	}
	return support.ParsedDeclaration{}, false
}

func assertCppMutationForTest(t *testing.T, analysis mutationAnalysis, target string, origin string) {
	t.Helper()
	if len(analysis.Mutations) == 0 {
		t.Fatalf("analysis = %#v, want %s/%s mutation", analysis, target, origin)
	}
	for _, evidence := range analysis.Mutations {
		if evidence.Target == target && evidence.Origin == origin {
			return
		}
	}
	t.Fatalf("mutations = %#v, want %s/%s", analysis.Mutations, target, origin)
}

func assertCppUnresolvedContains(t *testing.T, analysis mutationAnalysis, symbol string, operation string) {
	t.Helper()
	if len(analysis.Mutations) != 0 {
		t.Fatalf("mutations = %#v, want diagnostic-only evidence", analysis.Mutations)
	}
	for _, evidence := range analysis.Unresolved {
		if evidence.Language == "c++" && evidence.Symbol == symbol && evidence.Operation == operation && evidence.Line > 0 && evidence.Reason != "" {
			return
		}
	}
	t.Fatalf("unresolved = %#v, want c++ %s/%s evidence", analysis.Unresolved, symbol, operation)
}
