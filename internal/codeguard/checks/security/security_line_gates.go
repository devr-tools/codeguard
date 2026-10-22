package security

import (
	"regexp"
	"strings"
)

// PERF: the detectors in this package that run once per source line are
// (?i)/\b alternations, which is exactly the shape RE2 cannot accelerate — a
// leading \b or (?i) defeats its literal-prefix fast path, so the engine walks
// the whole line for every pattern on every line of every scanned file. A CPU
// profile of a full scan across seven real repositories put
// appendOWASPExtraLineFindings, isInsecureDeserialization and
// foldConcatenatedLiterals at the top of regexp.MatchString time.
//
// The fix is the one the secret scanner already uses (builtinGatePasses in
// security_secret_patterns.go): guard every regex with a cheap literal
// pre-check. strings.Contains is roughly 85x faster than the equivalent
// alternation, and a line holding none of a pattern's required literals cannot
// match it.
//
// INVARIANT: a gate must be a SUPERSET of the pattern it guards. A line the
// gate rejects never reaches the regex, so a missing literal silently disables
// a security rule — a false negative, the worst failure mode for a scanner.
// Two things keep that honest:
//
//   - gatedPattern owns its *regexp.Regexp, and matchString is the only way to
//     evaluate it, so a pattern cannot be run ungated by accident.
//   - newGatedPattern registers every instance in gatedPatterns, and
//     TestSecurityGatedPatternsAreSupersets walks that registry, deriving
//     concrete sample lines from each pattern's own regexp/syntax tree and
//     asserting the gate admits every one the pattern matches. A new pattern,
//     or a new alternation branch in an existing one, is checked automatically;
//     nothing about the test is hand-maintained.

// gatedPattern is a line regex plus the literal markers a line must contain for
// the regex to have any chance of matching.
//
// foldLiterals are compared ASCII-case-insensitively and must be given in
// lowercase; they are the markers for (?i) patterns. exactLiterals are compared
// byte-for-byte and are the markers for case-sensitive patterns. A line passes
// the gate when it contains any marker from either set.
type gatedPattern struct {
	name          string
	re            *regexp.Regexp
	foldLiterals  []string
	exactLiterals []string
}

// gatedPatterns holds every gated pattern in the package. newGatedPattern
// appends to it during package variable initialization, which is what makes
// the gate-coverage test exhaustive by construction rather than by review.
var gatedPatterns []*gatedPattern

// patternGate describes the literal markers for one pattern. Build one with
// foldGate or exactGate.
type patternGate struct {
	fold  []string
	exact []string
}

// foldGate declares markers matched case-insensitively; literals must be
// lowercase ASCII. Use it for patterns carrying (?i).
func foldGate(literals ...string) patternGate { return patternGate{fold: literals} }

// exactGate declares byte-exact markers. Use it for case-sensitive patterns.
func exactGate(literals ...string) patternGate { return patternGate{exact: literals} }

func newGatedPattern(name string, expr string, gate patternGate) *gatedPattern {
	pattern := &gatedPattern{
		name:          name,
		re:            regexp.MustCompile(expr),
		foldLiterals:  gate.fold,
		exactLiterals: gate.exact,
	}
	gatedPatterns = append(gatedPatterns, pattern)
	return pattern
}

// matchString reports whether line matches, running the regex only for lines
// the literal gate admits.
//
// ascii says whether line is known to hold no byte >= 0x80. Callers that test
// several patterns against the same line compute it once (see sourceLine);
// passing false is always safe because it only widens the gate.
func (p *gatedPattern) matchString(line string, ascii bool) bool {
	return p.gatePasses(line, ascii) && p.re.MatchString(line)
}

// gatePasses reports whether line could match. False positives cost a wasted
// regex run; false negatives would cost a finding, so every uncertainty here
// resolves to true.
func (p *gatedPattern) gatePasses(line string, ascii bool) bool {
	for _, literal := range p.exactLiterals {
		if strings.Contains(line, literal) {
			return true
		}
	}
	if len(p.foldLiterals) == 0 {
		return false
	}
	// Go's (?i) uses Unicode simple folding, so `s` also matches U+017F and `k`
	// also matches U+212A. Rather than model that, any line holding a non-ASCII
	// byte is admitted: a wasted regex run costs time, a skipped one costs a
	// security finding. Source lines are ASCII in practice, so the gate still
	// does its job. This mirrors quality.containsAnyFold.
	if !ascii {
		return true
	}
	for _, literal := range p.foldLiterals {
		if asciiContainsFold(line, literal) {
			return true
		}
	}
	return false
}

// anyGatePasses reports whether any of patterns admits line. It lets a caller
// skip work that is only needed when some pattern could fire.
func anyGatePasses(line string, ascii bool, patterns ...*gatedPattern) bool {
	for _, pattern := range patterns {
		if pattern.gatePasses(line, ascii) {
			return true
		}
	}
	return false
}

// sourceLine is one source line in both the forms the per-line detectors need,
// plus the ASCII flag the literal gates consume.
//
// Build it with newSourceLine: the flag is true only when BOTH forms hold
// nothing but ASCII, so every gate reading it stays a superset of its pattern
// even though masked and raw are scanned independently.
type sourceLine struct {
	raw    string
	masked string
	ascii  bool
}

func newSourceLine(raw string, masked string) sourceLine {
	return sourceLine{raw: raw, masked: masked, ascii: asciiOnly(raw) && asciiOnly(masked)}
}

// asciiOnly reports whether s holds no byte >= 0x80.
func asciiOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
