// Package suggest ranks known values against a mistyped one so error messages
// can offer "did you mean" hints for rule ids, profiles, output formats and
// config keys. It is deliberately conservative: a wrong suggestion is worse
// than none, so candidates must be within a small edit distance of the input or
// contain it as a dotted prefix/substring.
package suggest

import (
	"fmt"
	"sort"
	"strings"
)

// MaxSuggestions caps how many candidates a hint lists.
const MaxSuggestions = 3

// Distance returns the optimal-string-alignment edit distance between a and
// b, counted in runes: insertions, deletions, substitutions and transpositions
// of adjacent runes each cost one, so "jsno" is one edit from "json".
func Distance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	prevPrev := make([]int, len(br)+1)
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ar[i-1] == br[j-2] && ar[i-2] == br[j-1] {
				curr[j] = min(curr[j], prevPrev[j-2]+1)
			}
		}
		prevPrev, prev, curr = prev, curr, prevPrev
	}
	return prev[len(br)]
}

// maxDistance is the largest edit distance still treated as a typo: one edit
// for short inputs, growing to a third of the input length, never above 4.
func maxDistance(input string) int {
	n := len([]rune(input))
	return max(1, min(4, n/3))
}

type ranked struct {
	value string
	score int
}

// Closest returns up to MaxSuggestions candidates that plausibly match input,
// best first. Exact case-insensitive matches rank first, then candidates within
// the typo distance, then candidates that contain the input as a whole
// dot/dash-separated segment or substring (so "taint" finds "security.taint.go").
// Inputs shorter than three characters only match by edit distance.
func Closest(input string, candidates []string) []string {
	needle := strings.ToLower(strings.TrimSpace(input))
	if needle == "" || len(candidates) == 0 {
		return nil
	}
	limit := maxDistance(needle)
	seen := make(map[string]struct{}, len(candidates))
	matches := make([]ranked, 0, MaxSuggestions)
	for _, candidate := range candidates {
		if _, dup := seen[candidate]; dup || candidate == "" {
			continue
		}
		seen[candidate] = struct{}{}
		if score, ok := matchScore(needle, strings.ToLower(candidate), limit); ok {
			matches = append(matches, ranked{value: candidate, score: score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score < matches[j].score
		}
		return matches[i].value < matches[j].value
	})
	// A close typo match makes looser ones noise: keep only candidates within
	// one edit of the best, and drop containment matches entirely.
	cutoff := -1
	if len(matches) > 0 && matches[0].score < containmentScore {
		cutoff = matches[0].score + 1
	}
	out := make([]string, 0, min(len(matches), MaxSuggestions))
	for _, match := range matches {
		if len(out) == MaxSuggestions || (cutoff >= 0 && match.score > cutoff) {
			break
		}
		out = append(out, match.value)
	}
	return out
}

// containmentScore is the lowest score a segment/substring match can have;
// every edit-distance match scores below it.
const containmentScore = 10

// matchScore orders matches: lower is better. Edit-distance matches score
// their distance (0-4); segment matches score 10, substring matches 20, each
// plus the length difference so the tightest containing id wins.
func matchScore(needle string, candidate string, limit int) (int, bool) {
	if d := Distance(needle, candidate); d <= limit {
		return d, true
	}
	if len([]rune(needle)) < 3 || !strings.Contains(candidate, needle) {
		return 0, false
	}
	extra := len(candidate) - len(needle)
	if containsSegment(candidate, needle) {
		return containmentScore + extra, true
	}
	return 2*containmentScore + extra, true
}

func containsSegment(candidate string, needle string) bool {
	for _, segment := range strings.FieldsFunc(candidate, func(r rune) bool { return r == '.' || r == '-' || r == '_' }) {
		if segment == needle {
			return true
		}
	}
	return strings.HasPrefix(candidate, needle+".") || strings.HasPrefix(candidate, needle+"-")
}

// Hint formats the closest candidates as a sentence suffix, for example
// `; did you mean "strict"?` or `; did you mean one of "a", "b"?`. It returns
// "" when nothing is close enough, so callers can append it unconditionally.
func Hint(input string, candidates []string) string {
	return Format(Closest(input, candidates))
}

// Format renders an already-ranked suggestion list the way Hint does.
func Format(matches []string) string {
	switch len(matches) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("; did you mean %q?", matches[0])
	default:
		quoted := make([]string, len(matches))
		for i, match := range matches {
			quoted[i] = fmt.Sprintf("%q", match)
		}
		return "; did you mean one of " + strings.Join(quoted, ", ") + "?"
	}
}
