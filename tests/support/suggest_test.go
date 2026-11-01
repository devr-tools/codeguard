package support_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/suggest"
)

func TestSuggestDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"sarf", "sarif", 1},
		{"strct", "strict", 1},
		{"kitten", "sitting", 3},
		{"qualty", "quality", 1},
		{"jsno", "json", 1},
		{"ca", "abc", 3},
	}
	for _, tc := range cases {
		if got := suggest.Distance(tc.a, tc.b); got != tc.want {
			t.Errorf("Distance(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSuggestClosest(t *testing.T) {
	rules := []string{"security.taint.go", "security.taint.python", "security.taint.cpp", "security.taint.typescript", "quality.gofmt", "ci.lint"}
	cases := []struct {
		name       string
		input      string
		candidates []string
		want       []string
	}{
		{"typo within distance", "strct", []string{"startup", "strict", "enterprise", "ai-safe"}, []string{"strict"}},
		{"format typo", "sarf", []string{"text", "json", "sarif", "github"}, []string{"sarif"}},
		{"segment match lists at most three, shortest first", "taint", rules, []string{"security.taint.go", "security.taint.cpp", "security.taint.python"}},
		{"close typo drops looser matches", "security.taint.goo", rules, []string{"security.taint.go"}},
		{"bare rule name finds its section", "gofmt", rules, []string{"quality.gofmt"}},
		{"case insensitive", "STRICT", []string{"strict"}, []string{"strict"}},
		{"nothing close", "xyzzy", rules, nil},
		{"short input needs an edit-distance match", "go", rules, nil},
		{"empty input", "", rules, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := suggest.Closest(tc.input, tc.candidates)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Closest(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestSuggestHintFormatting(t *testing.T) {
	if got := suggest.Hint("xyzzy", []string{"strict"}); got != "" {
		t.Fatalf("Hint with no match = %q, want empty", got)
	}
	if got := suggest.Hint("strct", []string{"strict"}); got != `; did you mean "strict"?` {
		t.Fatalf("single hint = %q", got)
	}
	got := suggest.Format([]string{"a.b", "a.c"})
	if !strings.HasPrefix(got, "; did you mean one of ") || !strings.Contains(got, `"a.b", "a.c"`) {
		t.Fatalf("multi hint = %q", got)
	}
}
