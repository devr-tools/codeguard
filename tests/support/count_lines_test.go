package support_test

import (
	"strings"
	"testing"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// TestCountLinesMatchesLegacySemantics pins CountLines to the exact semantics
// of the allocation-heavy implementation it replaced
// (len(strings.Split(strings.TrimRight(string(data), "\n"), "\n"))): every
// trailing newline is trimmed before counting and empty input is one line.
// Each case is also cross-checked against that legacy formula so the table
// itself cannot drift from the historical behavior.
func TestCountLinesMatchesLegacySemantics(t *testing.T) {
	cases := []struct {
		name string
		data string
		want int
	}{
		{name: "empty", data: "", want: 1},
		{name: "single line no trailing newline", data: "a", want: 1},
		{name: "single line trailing newline", data: "a\n", want: 1},
		{name: "two lines no trailing newline", data: "a\nb", want: 2},
		{name: "two lines trailing newline", data: "a\nb\n", want: 2},
		{name: "interior blank line counts", data: "a\n\nb", want: 3},
		{name: "only one newline", data: "\n", want: 1},
		{name: "only newlines", data: "\n\n\n", want: 1},
		{name: "trailing blank lines trimmed", data: "a\n\n\n", want: 1},
		{name: "leading newline counts", data: "\na", want: 2},
		{name: "leading blank lines count", data: "\n\nx", want: 3},
		{name: "crlf keeps carriage return", data: "a\r\nb\r\n", want: 2},
		{name: "lone carriage return is not a newline", data: "a\rb", want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			legacy := len(strings.Split(strings.TrimRight(tc.data, "\n"), "\n"))
			if legacy != tc.want {
				t.Fatalf("test table drifted from legacy semantics: legacy(%q) = %d, table wants %d", tc.data, legacy, tc.want)
			}
			if got := runnersupport.CountLines([]byte(tc.data)); got != tc.want {
				t.Fatalf("CountLines(%q) = %d, want %d", tc.data, got, tc.want)
			}
		})
	}
}
