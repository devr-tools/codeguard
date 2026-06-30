package support_test

import (
	"testing"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// TestValidateBaseRef locks in the option-injection guard added to
// runner/support: refs that git could misparse as an option, or that contain
// characters outside the conservative ref/SHA charset, must be rejected, while
// legitimate refs (and the "stdin" sentinel) must pass.
func TestValidateBaseRef(t *testing.T) {
	cases := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		// Rejections: leading '-' would be parsed by git as an option.
		{"option_upload_pack", "--upload-pack=evil", true},
		{"short_option", "-x", true},
		{"single_dash", "-", true},
		// Rejections: characters outside the allowed charset.
		{"empty", "", true},
		{"space", "main branch", true},
		{"semicolon", "main;rm -rf", true},
		{"dollar", "main$(whoami)", true},
		{"backtick", "main`id`", true},
		{"newline", "main\nHEAD", true},
		{"star", "main*", true},
		{"backslash", `main\HEAD`, true},
		{"question", "main?", true},
		{"hash", "main#1", true},

		// Acceptances: legitimate refs.
		{"branch", "main", false},
		{"remote_branch", "origin/main", false},
		{"ancestor", "HEAD~3", false},
		{"tag", "v1.2.3", false},
		{"sha40", "0123456789abcdef0123456789abcdef01234567", false},
		{"feature_branch", "feature/foo-bar", false},
		{"caret_parent", "HEAD^", false},
		{"reflog_at", "main@{1}", false},
		{"ref_colon_path", "HEAD:path/to/file", false},
		{"stdin_sentinel", "stdin", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runnersupport.ValidateBaseRef(tc.ref)
			if tc.wantErr && err == nil {
				t.Fatalf("ValidateBaseRef(%q) = nil, want error", tc.ref)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateBaseRef(%q) = %v, want nil", tc.ref, err)
			}
		})
	}
}

// safeMatchPattern calls MatchPattern, converting any panic into a test
// failure. The hardening under test guarantees an un-compilable glob returns
// false rather than panicking the scan.
func safeMatchPattern(t *testing.T, pattern, value string) (got bool) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("MatchPattern(%q, %q) panicked: %v", pattern, value, r)
		}
	}()
	return runnersupport.MatchPattern(pattern, value)
}

// TestMatchPatternMalformed verifies that globs which translate to an invalid
// regex match nothing rather than panicking the walk.
func TestMatchPatternMalformed(t *testing.T) {
	// `\x` and a trailing stray backslash both produce invalid regex after the
	// glob->regex translation; they must be handled gracefully.
	malformed := []string{
		`\x`,
		`foo\`,
		`bad\q`,
	}
	for _, pattern := range malformed {
		t.Run(pattern, func(t *testing.T) {
			if got := safeMatchPattern(t, pattern, "foo/bar"); got {
				t.Fatalf("MatchPattern(%q, ...) = true, want false for malformed glob", pattern)
			}
		})
	}
}

// TestMatchPatternValid verifies the documented glob semantics: '*' matches
// within a path segment, '**' crosses separators, '?' matches a single
// non-separator char, and exact paths match literally.
func TestMatchPatternValid(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{"star_all_single_segment", "*", "main.go", true},
		{"star_not_across_slash", "*", "dir/main.go", false},
		{"star_suffix", "*.go", "main.go", true},
		{"star_suffix_no_match", "*.go", "main.py", false},
		{"doublestar_crosses_slash", "**", "a/b/c.go", true},
		{"dir_doublestar_match", "node_modules/**", "node_modules/pkg/index.js", true},
		{"dir_doublestar_no_match", "node_modules/**", "src/node_modules.go", false},
		{"question_single_char", "?.go", "a.go", true},
		{"question_not_two_chars", "?.go", "ab.go", false},
		{"question_not_slash", "a?b", "a/b", false},
		{"exact_path", "src/main.go", "src/main.go", true},
		{"exact_path_no_match", "src/main.go", "src/other.go", false},
		{"dot_is_literal", "a.b", "axb", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeMatchPattern(t, tc.pattern, tc.value); got != tc.want {
				t.Fatalf("MatchPattern(%q, %q) = %v, want %v", tc.pattern, tc.value, got, tc.want)
			}
		})
	}
}

// TestMatchPatternIdempotent exercises the sync.Map memoization: repeated calls
// with the same pattern must return a stable result (the second call is served
// from the compiled-pattern cache).
func TestMatchPatternIdempotent(t *testing.T) {
	cases := []struct {
		pattern string
		value   string
	}{
		{"node_modules/**", "node_modules/pkg/index.js"},
		{"*.go", "main.go"},
		{`\x`, "anything"}, // cached as nil (failed compile); still stable.
	}
	for _, tc := range cases {
		t.Run(tc.pattern, func(t *testing.T) {
			first := safeMatchPattern(t, tc.pattern, tc.value)
			second := safeMatchPattern(t, tc.pattern, tc.value)
			if first != second {
				t.Fatalf("MatchPattern(%q, %q) not idempotent: first=%v second=%v",
					tc.pattern, tc.value, first, second)
			}
		})
	}
}
