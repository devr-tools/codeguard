package security

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestAppendOWASPExtraLineFindingsSuppressesCommentOnlyMatches(t *testing.T) {
	env := testSecurityContext()
	for _, tc := range []struct {
		name   string
		raw    string
		masked string
	}{
		{name: "bind all", raw: "// 0.0.0.0 is mentioned in documentation"},
		{name: "cors wildcard", raw: "# Access-Control-Allow-Origin: * in an example", masked: "# Access-Control-Allow-Origin: * in an example"},
		{name: "weak hash", raw: "// crypto/md5 is discussed in a migration note"},
		{name: "weak cipher", raw: "/* cipher.getInstance(\"DES\") appears in prose */"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings := appendOWASPExtraLineFindings(env, "fixture.go", 1, newSourceLine(tc.raw, tc.masked))
			if len(findings) != 0 {
				t.Fatalf("findings = %#v, want none", findings)
			}
		})
	}
}

func TestAppendOWASPExtraLineFindingsKeepsRealMatches(t *testing.T) {
	env := testSecurityContext()
	for _, tc := range []struct {
		name   string
		file   string
		raw    string
		masked string
		rule   string
	}{
		{name: "bind all", file: "server.yaml", raw: `host: "0.0.0.0"`, masked: `host: "0.0.0.0"`, rule: "security.bind-all-interfaces"},
		{name: "cors wildcard", file: "headers.yaml", raw: `Access-Control-Allow-Origin: *`, masked: `Access-Control-Allow-Origin: *`, rule: "security.cors-wildcard"},
		{name: "weak hash", file: "digest.go", raw: `sum := md5.New()`, masked: `sum := md5.New()`, rule: "security.weak-hash"},
		{name: "weak cipher", file: "crypto.java", raw: `Cipher.getInstance("DES")`, masked: `Cipher.getInstance("DES")`, rule: "security.weak-cipher"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings := appendOWASPExtraLineFindings(env, tc.file, 7, newSourceLine(tc.raw, tc.masked))
			if len(findings) != 1 {
				t.Fatalf("findings = %#v, want exactly one", findings)
			}
			if findings[0].RuleID != tc.rule || findings[0].Line != 7 {
				t.Fatalf("finding = %#v, want %s at line 7", findings[0], tc.rule)
			}
		})
	}
}

func TestAppendOWASPExtraLineFindingsSuppressesTrailingCommentMatches(t *testing.T) {
	env := testSecurityContext()
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{name: "weak hash", raw: `return nil // md5.New() was removed`},
		{name: "weak cipher", raw: `return nil /* cipher.getInstance("DES") was removed */`},
		{name: "cors wildcard", raw: `return nil // Access-Control-Allow-Origin: * example`},
		{name: "bind all", raw: `return nil // 0.0.0.0 example`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings := appendOWASPExtraLineFindings(env, "fixture.go", 1, newSourceLine(tc.raw, maskedSourceForFile("fixture.go", tc.raw)))
			if len(findings) != 0 {
				t.Fatalf("findings = %#v, want none", findings)
			}
		})
	}
}

func TestFindingsForFileSuppressesConfigCommentOnlyMatches(t *testing.T) {
	env := testSecurityContext()
	for _, tc := range []struct {
		name string
		file string
		data string
	}{
		{name: "yaml cors", file: "headers.yaml", data: "# Access-Control-Allow-Origin: * example\n"},
		{name: "env bind all", file: ".env", data: "# HOST=0.0.0.0\n"},
		{name: "docker weak hash", file: "Dockerfile", data: "# crypto/md5 was removed\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings := findingsForFile(env, tc.file, []byte(tc.data))
			if len(findings) != 0 {
				t.Fatalf("findings = %#v, want none", findings)
			}
		})
	}
}

func testSecurityContext() support.Context {
	return support.Context{NewFinding: func(input support.FindingInput) core.Finding {
		return core.Finding{
			RuleID:  input.RuleID,
			Level:   input.Level,
			Path:    input.Path,
			Line:    input.Line,
			Column:  input.Column,
			Message: input.Message,
		}
	}}
}

// --- literal gate coverage -------------------------------------------------
//
// These tests are the enforcement half of the superset invariant documented in
// security_line_gates.go: every gate must admit every line its pattern can
// match, because a line the gate rejects never reaches the regex and a missed
// match is a silent false negative in a security scanner.
//
// They are deliberately mechanical. Rather than list the literals a pattern
// requires (a list that drifts the moment someone adds an alternation branch),
// regexpSampleBranches reads the pattern's own regexp/syntax tree and
// enumerates concrete strings it accepts: every alternation branch, every
// member of a small character class (the parser factors `(?:ghp|gho|ghu)` into
// `gh[opu]`, so branches hide there), the zero- and one-repetition forms of
// every optional element, and the minimum plus one of every bounded one.
// Adding a pattern or a branch therefore extends the test automatically.
//
// The expansion is an under-approximation of the pattern's language, so it is
// evidence rather than proof — but it is evidence that regenerates itself, and
// it catches the realistic failure: a new branch whose marker nobody added to
// the gate.

func TestSecurityGatedPatternsAreSupersets(t *testing.T) {
	if len(gatedPatterns) == 0 {
		t.Fatal("no gated patterns registered; newGatedPattern is the only way to build one")
	}
	seen := map[string]bool{}
	for _, pattern := range gatedPatterns {
		if seen[pattern.name] {
			t.Fatalf("duplicate gated pattern name %q", pattern.name)
		}
		seen[pattern.name] = true
		if len(pattern.foldLiterals) == 0 && len(pattern.exactLiterals) == 0 {
			t.Fatalf("gated pattern %q declares no literals, so its gate rejects every line", pattern.name)
		}
		for _, literal := range pattern.foldLiterals {
			if literal != strings.ToLower(literal) {
				t.Fatalf("gated pattern %q fold literal %q must be lowercase", pattern.name, literal)
			}
		}
		t.Run(pattern.name, func(t *testing.T) {
			assertGateAdmitsPattern(t, pattern.re, pattern.name, func(line string) bool {
				return pattern.gatePasses(line, asciiOnly(line))
			})
		})
	}
}

// assertGateAdmitsPattern checks that gate admits every generated line re
// matches, and that every top-level alternation branch contributed at least one
// such line, so a branch the generator cannot reach is reported rather than
// silently vouched for.
func assertGateAdmitsPattern(t *testing.T, re *regexp.Regexp, name string, gate func(string) bool) {
	t.Helper()
	for idx, samples := range regexpSampleBranches(t, re.String()) {
		matched := 0
		for _, sample := range samples {
			// Both case folds are tried because a pattern carrying (?i) matches
			// either, so a byte-exact gate literal on such a pattern would be a
			// real bug. Variants the pattern does not match are skipped.
			for _, variant := range []string{sample, strings.ToLower(sample), strings.ToUpper(sample)} {
				if !re.MatchString(variant) {
					continue
				}
				matched++
				if !gate(variant) {
					t.Fatalf("%s: gate rejects %q, which the pattern matches\npattern: %s\nadd the missing literal to its gate", name, variant, re)
				}
			}
		}
		if matched == 0 {
			t.Fatalf("%s: alternation branch %d produced no matching sample, so its gate is unverified\npattern: %s", name, idx, re)
		}
	}
}

// maxRegexpSamples bounds the expansion. Exceeding it fails the test rather
// than silently sampling, so a pattern that outgrows the generator is noticed.
const maxRegexpSamples = 50000

// regexpSampleBranches returns, per top-level alternation branch of expr, the
// concrete strings that branch accepts.
func regexpSampleBranches(t *testing.T, expr string) [][]string {
	t.Helper()
	parsed, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		t.Fatalf("parse %s: %v", expr, err)
	}
	branches := []*syntax.Regexp{parsed}
	if parsed.Op == syntax.OpAlternate {
		branches = parsed.Sub
	}
	out := make([][]string, 0, len(branches))
	for _, branch := range branches {
		samples, err := expandRegexpSamples(branch)
		if err != nil {
			t.Fatalf("expand %s: %v", expr, err)
		}
		out = append(out, samples)
	}
	return out
}

func expandRegexpSamples(re *syntax.Regexp) ([]string, error) {
	switch re.Op {
	case syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine,
		syntax.OpBeginText, syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return []string{""}, nil
	case syntax.OpNoMatch:
		return nil, nil
	case syntax.OpLiteral:
		return []string{string(re.Rune)}, nil
	case syntax.OpCharClass:
		runes := classSampleRunes(re.Rune)
		if len(runes) == 0 {
			return nil, fmt.Errorf("character class with no runes")
		}
		samples := make([]string, 0, len(runes))
		for _, r := range runes {
			samples = append(samples, string(r))
		}
		return samples, nil
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		return []string{"x"}, nil
	case syntax.OpCapture:
		return expandRegexpSamples(re.Sub[0])
	case syntax.OpConcat:
		return expandConcatSamples(re.Sub)
	case syntax.OpAlternate:
		var out []string
		for _, sub := range re.Sub {
			samples, err := expandRegexpSamples(sub)
			if err != nil {
				return nil, err
			}
			out = append(out, samples...)
		}
		return cappedSamples(out)
	case syntax.OpStar, syntax.OpQuest:
		return expandRepeatSamples(re.Sub[0], 0, 1)
	case syntax.OpPlus:
		return expandRepeatSamples(re.Sub[0], 1, 2)
	case syntax.OpRepeat:
		high := re.Min + 1
		if re.Max >= 0 && high > re.Max {
			high = re.Max
		}
		return expandRepeatSamples(re.Sub[0], re.Min, high)
	default:
		return nil, fmt.Errorf("unsupported regexp op %v", re.Op)
	}
}

// expandRepeatSamples covers the counts that can change whether a gate literal
// stays contiguous: none, one, and one more than the minimum. Counts above one
// use a single representative expansion, since all they test is that a literal
// does not span the repeated element.
func expandRepeatSamples(sub *syntax.Regexp, low int, high int) ([]string, error) {
	samples, err := expandRegexpSamples(sub)
	if err != nil {
		return nil, err
	}
	if len(samples) == 0 {
		return []string{""}, nil
	}
	var out []string
	for count := low; count <= high; count++ {
		switch count {
		case 0:
			out = append(out, "")
		case 1:
			out = append(out, samples...)
		default:
			out = append(out, strings.Repeat(samples[0], count))
		}
	}
	return cappedSamples(out)
}

func expandConcatSamples(subs []*syntax.Regexp) ([]string, error) {
	out := []string{""}
	for _, sub := range subs {
		parts, err := expandRegexpSamples(sub)
		if err != nil {
			return nil, err
		}
		if len(parts) == 0 {
			parts = []string{""}
		}
		next := make([]string, 0, len(out)*len(parts))
		for _, prefix := range out {
			for _, part := range parts {
				next = append(next, prefix+part)
			}
		}
		if out, err = cappedSamples(next); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func cappedSamples(samples []string) ([]string, error) {
	if len(samples) > maxRegexpSamples {
		return nil, fmt.Errorf("expands to more than %d samples; split the pattern or extend the generator", maxRegexpSamples)
	}
	return samples, nil
}

// classSampleRunes returns the runes to generate for a character class. Small
// identifier-shaped classes are expanded in full because the parser factors
// alternations such as (?:ghp|gho|ghu) into gh[opu], and each of those really
// is a separate branch needing its own gate literal. Everything else gets one
// representative rune, preferring a printable ASCII one.
func classSampleRunes(ranges []rune) []rune {
	var all []rune
	for i := 0; i+1 < len(ranges); i += 2 {
		for r := ranges[i]; r <= ranges[i+1]; r++ {
			all = append(all, r)
			if len(all) > 8 {
				return representativeRunes(ranges)
			}
		}
	}
	for _, r := range all {
		if !isIdentifierRune(r) {
			return representativeRunes(ranges)
		}
	}
	if len(all) == 0 {
		return nil
	}
	return all
}

func isIdentifierRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-'
}

// representativeRunes prefers a word character across the whole class before
// settling for any printable one: picking '-' out of [0-9A-Za-z_-] would end a
// generated sample on a non-word byte and silently defeat a trailing \b, which
// makes the sample unmatchable and the gate untested.
func representativeRunes(ranges []rune) []rune {
	for _, preferred := range []rune{'a', 'A', '0', ' '} {
		for i := 0; i+1 < len(ranges); i += 2 {
			if preferred >= ranges[i] && preferred <= ranges[i+1] {
				return []rune{preferred}
			}
		}
	}
	for i := 0; i+1 < len(ranges); i += 2 {
		lo, hi := ranges[i], ranges[i+1]
		if r := max(lo, '!'); hi >= '!' && lo <= '~' && r <= hi {
			return []rune{r}
		}
	}
	if len(ranges) >= 2 {
		return []rune{ranges[0]}
	}
	return nil
}
