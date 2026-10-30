package support

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// inlineSuppressionIndex holds every inline directive of one file, parsed once
// per scan, plus a line index so a finding is matched by looking at the two
// lines that can cover it instead of re-reading and re-scanning the file.
//
// directives keeps source order, which decides precedence: the original
// matcher walked the list and took the first directive that covered the
// finding, and the winner's match label ends up in the --include-suppressed
// evidence. byLine maps a directive's covered line to its positions in
// directives, so match can pick the lowest position among the candidates and
// reproduce that first-match result exactly.
type inlineSuppressionIndex struct {
	directives []inlineSuppression
	byLine     map[int][]int
}

// emptyInlineSuppressions is shared by every file that has no directive at all,
// which is nearly every file a finding lands in.
var emptyInlineSuppressions = &inlineSuppressionIndex{}

type inlineSuppressionEntry struct {
	once  sync.Once
	index *inlineSuppressionIndex
}

// inlineSuppressionCache memoizes inlineSuppressionIndex per finding path for
// one scan. It lives on the fileCorpus so every by-value Context copy shares it.
type inlineSuppressionCache struct {
	mu      sync.Mutex
	entries map[string]*inlineSuppressionEntry
}

// inlineSuppressionsFor returns the directive index for the file a finding's
// target-relative path resolves to, or nil when the path resolves to no
// readable file (in which case no inline suppression can apply). With a
// corpus attached the resolution, read, and parse happen once per path per
// scan; without one (a bare Context in tests) they happen on every call, as
// they always did.
func inlineSuppressionsFor(sc Context, rel string) *inlineSuppressionIndex {
	if rel == "" {
		return nil
	}
	if sc.corpus == nil {
		return loadInlineSuppressions(sc, rel)
	}
	cache := &sc.corpus.suppressions
	key := inlineSuppressionKey(sc, rel)
	cache.mu.Lock()
	if cache.entries == nil {
		cache.entries = map[string]*inlineSuppressionEntry{}
	}
	entry, ok := cache.entries[key]
	if !ok {
		entry = &inlineSuppressionEntry{}
		cache.entries[key] = entry
	}
	cache.mu.Unlock()
	entry.once.Do(func() {
		entry.index = loadInlineSuppressions(sc, rel)
	})
	return entry.index
}

// inlineSuppressionKey includes the target roots because resolution depends on
// them; within one scan they never change, so this costs nothing in hit rate.
func inlineSuppressionKey(sc Context, rel string) string {
	var b strings.Builder
	b.WriteString(rel)
	for _, target := range sc.Cfg.Targets {
		b.WriteByte(0)
		b.WriteString(target.Path)
	}
	return b.String()
}

// loadInlineSuppressions resolves rel exactly as the per-finding lookup always
// has (the first target where the joined path exists), reads it, and parses
// its directives. Bytes the scan already holds in the corpus are reused; any
// other file is read directly, uncapped and without being retained, so this
// lookup never consumes corpus budget (which could otherwise surface a new
// budget diagnostic) and never changes what a large file reads as.
func loadInlineSuppressions(sc Context, rel string) *inlineSuppressionIndex {
	root, fullPath := findingTargetPath(sc, rel)
	if fullPath == "" {
		return nil
	}
	data, ok := sc.corpus.cachedRead(root, filepath.FromSlash(rel))
	if !ok {
		var err error
		data, err = os.ReadFile(fullPath) //nolint:gosec // path resolved via findingTargetPath against the scan context
		if err != nil {
			return nil
		}
	}
	return indexInlineSuppressions(fullPath, data)
}

// findingTargetPath returns the first target root under which rel exists and
// the joined path, or empty strings when it exists under none.
func findingTargetPath(sc Context, rel string) (string, string) {
	if rel == "" {
		return "", ""
	}
	for _, target := range sc.Cfg.Targets {
		candidate := filepath.Join(target.Path, filepath.FromSlash(rel))
		if _, err := os.Stat(candidate); err == nil {
			return target.Path, candidate
		}
	}
	return "", ""
}

var (
	codeguardIgnoreMarker = []byte("codeguard:ignore")
	nolintMarker          = []byte("nolint:")
	nosecMarker           = []byte("#nosec")
)

// indexInlineSuppressions parses every directive in data. A file without any
// directive marker skips the line split and every regex: inlineIgnorePattern
// cannot match without the literal "codeguard:ignore", and
// gosecJustifiedSuppressions already returns nothing for a line containing
// neither "nolint:" nor "#nosec".
func indexInlineSuppressions(path string, data []byte) *inlineSuppressionIndex {
	goSource := strings.EqualFold(filepath.Ext(path), ".go")
	hasIgnore := bytes.Contains(data, codeguardIgnoreMarker)
	hasGosec := goSource && (bytes.Contains(data, nolintMarker) || bytes.Contains(data, nosecMarker))
	if !hasIgnore && !hasGosec {
		return emptyInlineSuppressions
	}
	directives := parseInlineSuppressionLines(strings.Split(string(data), "\n"), hasIgnore, hasGosec)
	if len(directives) == 0 {
		return emptyInlineSuppressions
	}
	index := &inlineSuppressionIndex{directives: directives, byLine: make(map[int][]int, len(directives))}
	for position, directive := range directives {
		index.byLine[directive.line] = append(index.byLine[directive.line], position)
	}
	return index
}

func parseInlineSuppressionLines(lines []string, hasIgnore bool, hasGosec bool) []inlineSuppression {
	out := make([]inlineSuppression, 0) //nolint:prealloc // directive count is unknown until each line is matched
	for idx, line := range lines {
		if hasIgnore && strings.Contains(line, "codeguard:ignore") {
			for _, match := range inlineIgnorePattern.FindAllStringSubmatch(line, -1) {
				out = append(out, inlineSuppression{
					ruleID:  match[1],
					line:    idx + 1,
					expires: match[2],
				})
			}
		}
		if hasGosec {
			out = append(out, gosecJustifiedSuppressions(line, idx+1)...)
		}
	}
	return out
}

// match returns the first directive, in source order, that covers finding.
// Coverage is unchanged from the linear matcher: a codeguard:ignore directive
// covers its own line and the next (and any file-level finding, line 0); a
// translated gosec directive covers exactly its target line. Rule and expiry
// are checked at match time because expiry depends on the scan date.
func (index *inlineSuppressionIndex) match(sc Context, finding core.Finding) (inlineSuppression, bool) {
	if len(index.directives) == 0 {
		return inlineSuppression{}, false
	}
	if finding.Line == 0 {
		for _, directive := range index.directives {
			if !directive.exactLine && directive.appliesTo(sc, finding) {
				return directive, true
			}
		}
		return inlineSuppression{}, false
	}
	best := -1
	for _, position := range index.byLine[finding.Line] {
		if (best < 0 || position < best) && index.directives[position].appliesTo(sc, finding) {
			best = position
		}
	}
	for _, position := range index.byLine[finding.Line-1] {
		directive := index.directives[position]
		if !directive.exactLine && (best < 0 || position < best) && directive.appliesTo(sc, finding) {
			best = position
		}
	}
	if best < 0 {
		return inlineSuppression{}, false
	}
	return index.directives[best], true
}

func (directive inlineSuppression) appliesTo(sc Context, finding core.Finding) bool {
	if directive.ruleID != "*" && directive.ruleID != finding.RuleID {
		return false
	}
	return !suppressionExpired(directive.expires, sc.Today)
}
