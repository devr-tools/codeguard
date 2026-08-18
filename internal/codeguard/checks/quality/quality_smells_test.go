package quality

import (
	"reflect"
	"testing"
)

func TestCLikeClassMembersTrackBraceDepthInSinglePass(t *testing.T) {
	body := "\n  string first;\n  void nested() {\n    string first;\n  }\n  string second;\n"

	if got, want := clikeClassFields(body), []string{"first", "second"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("clikeClassFields() = %v, want %v", got, want)
	}

	methods := clikeClassMethods(body, body, 10, "Example")
	if len(methods) != 1 {
		t.Fatalf("clikeClassMethods() returned %d methods, want 1", len(methods))
	}
	if got, want := methods[0].StartLine, 12; got != want {
		t.Errorf("method start line = %d, want %d", got, want)
	}
	if got, want := methods[0].EndLine, 14; got != want {
		t.Errorf("method end line = %d, want %d", got, want)
	}
}
