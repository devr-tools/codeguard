package support

import "testing"

func TestCppParserRecordsInferredAliasesCapturesAndQualifiedMembers(t *testing.T) {
	parsed := ParseCLike(`namespace N {
struct Counter {
  Token state;
  int current(Token* inputPtr);
};
}

int N::Counter::current(Token* inputPtr) {
  auto first = inputPtr;
  auto second = &state;
  auto work = [=]() { return first->value; };
  return second->value;
}`, CLikeCPP)
	fn := parsed.FunctionByName("N::Counter::current")
	if fn == nil {
		t.Fatal("qualified function missing")
	}
	if fn.QualifiedOwner != "N::Counter" {
		t.Fatalf("qualified owner = %q, want N::Counter", fn.QualifiedOwner)
	}
	for name, alias := range map[string]string{"first": "inputPtr", "second": "state"} {
		declaration, ok := cppParsedDeclarationForTest(fn.Declarations, name, "local")
		if !ok || declaration.AliasSource != alias {
			t.Fatalf("declaration %q = %#v, ok=%v, want alias %q", name, declaration, ok, alias)
		}
	}
	capture, ok := cppParsedDeclarationForTest(fn.Declarations, "*", "capture")
	if !ok || capture.ReferenceShape != "value" {
		t.Fatalf("default capture = %#v, ok=%v, want value capture", capture, ok)
	}
	member, ok := cppParsedDeclarationForTest(fn.Declarations, "state", "member")
	if !ok || member.QualifiedOwner != "N::Counter" {
		t.Fatalf("member = %#v, ok=%v, want N::Counter-qualified state", member, ok)
	}
}

func TestCppParserScopesLambdaLocalsAndBuildsNestedNamespaceOwner(t *testing.T) {
	parsed := ParseCLike(`namespace N {
namespace M {
struct Counter {
  Token state;
  int current();
};
}
}

int N::M::Counter::current() {
  auto work = [=]() {
    Token* p = &state;
    p->value++;
  };
  work();
  return state.value;
}`, CLikeCPP)
	fn := parsed.FunctionByName("N::M::Counter::current")
	if fn == nil {
		t.Fatal("nested namespace function missing")
	}
	if fn.QualifiedOwner != "N::M::Counter" {
		t.Fatalf("qualified owner = %q, want N::M::Counter", fn.QualifiedOwner)
	}
	member, ok := cppParsedDeclarationForTest(fn.Declarations, "state", "member")
	if !ok || member.QualifiedOwner != "N::M::Counter" {
		t.Fatalf("member = %#v, ok=%v, want N::M::Counter-qualified state", member, ok)
	}
	local, ok := cppParsedDeclarationForTest(fn.Declarations, "p", "local")
	if !ok || local.AliasSource != "state" || local.Line < local.ScopeStart || local.ScopeEnd < local.Line {
		t.Fatalf("lambda local = %#v, ok=%v, want scoped pointer alias to state", local, ok)
	}
}

func cppParsedDeclarationForTest(declarations []ParsedDeclaration, name string, kind string) (ParsedDeclaration, bool) {
	for _, declaration := range declarations {
		if declaration.Name == name && declaration.Kind == kind {
			return declaration, true
		}
	}
	return ParsedDeclaration{}, false
}
