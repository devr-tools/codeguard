package support

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/rules"
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
	// invalid holds codeguard:ignore directives whose rule selector uses "*"
	// other than as a bare wildcard or a trailing ".*" prefix. They never
	// suppress; invalidCovering surfaces them so the author learns that.
	invalid []inlineSuppression
	// peers holds the directive indexes of the same target-relative path
	// under every other target where it exists, for a finding whose producing
	// pass did not record its target (core.Finding.TargetPath). With targets
	// a/ and b/ both holding main.go such a finding's own file cannot be told
	// apart, so a directive suppresses only when every candidate file has one
	// covering the finding: an ignore in a/main.go cannot hide a finding in
	// b/main.go.
	peers []*inlineSuppressionIndex
	// usage records which directives covered a finding in this scan, for the
	// suppression.unused-directive diagnostic. It is nil for an index with no
	// directives.
	usage *directiveUsage
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

// inlineSuppressionsForFinding returns the directive index that governs
// finding: its own target's file when the producing pass recorded the target,
// otherwise whatever its target-relative path resolves to (see
// inlineSuppressionsFor).
func inlineSuppressionsForFinding(sc Context, finding core.Finding) *inlineSuppressionIndex {
	if finding.TargetPath == "" || finding.Path == "" {
		return inlineSuppressionsFor(sc, finding.Path)
	}
	candidate := targetFilePath{
		root: finding.TargetPath,
		path: filepath.Join(finding.TargetPath, filepath.FromSlash(finding.Path)),
	}
	if _, err := os.Stat(candidate.path); err != nil {
		return nil
	}
	return fileInlineSuppressions(sc, finding.Path, candidate)
}

// inlineSuppressionsFor returns the directive index for the file a
// target-relative path resolves to, or nil when it resolves to no readable
// file (in which case no inline suppression can apply). When the path exists
// under several targets the index keeps the other files as peers, so a
// directive suppresses only when every candidate file has one.
func inlineSuppressionsFor(sc Context, rel string) *inlineSuppressionIndex {
	if rel == "" {
		return nil
	}
	candidates := findingTargetPaths(sc, rel)
	if len(candidates) == 0 {
		return nil
	}
	primary := fileInlineSuppressions(sc, rel, candidates[0])
	if primary == nil || len(candidates) == 1 || len(primary.directives) == 0 {
		return primary
	}
	return memoizedInlineSuppressions(sc, "peers\x00"+inlineSuppressionKey(sc, rel), func() *inlineSuppressionIndex {
		resolved := *primary
		resolved.peers = make([]*inlineSuppressionIndex, 0, len(candidates)-1)
		for _, candidate := range candidates[1:] {
			peer := fileInlineSuppressions(sc, rel, candidate)
			if peer == nil {
				peer = emptyInlineSuppressions
			}
			resolved.peers = append(resolved.peers, peer)
		}
		return &resolved
	})
}

// fileInlineSuppressions returns the directive index of one physical file,
// parsed once per scan. Every lookup of that file shares the index, so the
// directive usage recorded on it is complete whichever path led to it.
func fileInlineSuppressions(sc Context, rel string, candidate targetFilePath) *inlineSuppressionIndex {
	return memoizedInlineSuppressions(sc, "file\x00"+filepath.Clean(candidate.path), func() *inlineSuppressionIndex {
		return readInlineSuppressions(sc, rel, candidate)
	})
}

// memoizedInlineSuppressions runs load once per key per scan. With no corpus
// attached (a bare Context in tests) it runs load on every call.
func memoizedInlineSuppressions(sc Context, key string, load func() *inlineSuppressionIndex) *inlineSuppressionIndex {
	if sc.corpus == nil {
		return load()
	}
	cache := &sc.corpus.suppressions
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
		entry.index = load()
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

// readInlineSuppressions reads and parses one candidate file. Bytes the scan
// already holds in the corpus are reused; any other file is read directly,
// uncapped and without being retained, so this lookup never consumes corpus
// budget (which could otherwise surface a new budget diagnostic) and never
// changes what a large file reads as.
func readInlineSuppressions(sc Context, rel string, candidate targetFilePath) *inlineSuppressionIndex {
	data, ok := sc.corpus.cachedRead(candidate.root, filepath.FromSlash(rel))
	if !ok {
		var err error
		data, err = os.ReadFile(candidate.path) //nolint:gosec // path resolved via findingTargetPaths against the scan context
		if err != nil {
			return nil
		}
	}
	return indexInlineSuppressions(candidate.path, data)
}

type targetFilePath struct {
	root string
	path string
}

// findingTargetPaths returns, in target order, every target root under which
// rel exists, with the joined path.
func findingTargetPaths(sc Context, rel string) []targetFilePath {
	if rel == "" {
		return nil
	}
	var out []targetFilePath
	for _, target := range sc.Cfg.Targets {
		candidate := filepath.Join(target.Path, filepath.FromSlash(rel))
		if _, err := os.Stat(candidate); err == nil {
			out = append(out, targetFilePath{root: target.Path, path: candidate})
		}
	}
	return out
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
	parsed := parseInlineSuppressionLines(strings.Split(string(data), "\n"), hasIgnore, hasGosec)
	directives := make([]inlineSuppression, 0, len(parsed))
	var invalid []inlineSuppression
	for _, directive := range parsed {
		if kind, _ := rules.ClassifyRulePattern(directive.ruleID); kind == rules.RulePatternInvalid {
			invalid = append(invalid, directive)
			continue
		}
		directives = append(directives, directive)
	}
	if len(directives) == 0 && len(invalid) == 0 {
		return emptyInlineSuppressions
	}
	index := &inlineSuppressionIndex{
		directives: directives,
		byLine:     make(map[int][]int, len(directives)),
		invalid:    invalid,
		usage:      &directiveUsage{used: make([]bool, len(directives))},
	}
	for position, directive := range directives {
		index.byLine[directive.line] = append(index.byLine[directive.line], position)
	}
	return index
}

// parseIgnoreDirectives returns the codeguard:ignore directives on one line.
// The reason of each is the text between the end of its rule id (or until
// date) and the next directive on the line, or the end of the line.
func parseIgnoreDirectives(line string, lineNo int) []inlineSuppression {
	matches := inlineIgnorePattern.FindAllStringSubmatchIndex(line, -1)
	out := make([]inlineSuppression, 0, len(matches))
	for i, match := range matches {
		tailEnd := len(line)
		if i+1 < len(matches) {
			tailEnd = matches[i+1][0]
		}
		directive := inlineSuppression{
			ruleID: line[match[2]:match[3]],
			line:   lineNo,
			reason: directiveReason(line[match[1]:tailEnd]),
			quoted: insideStringLiteral(line[:match[0]]),
		}
		if match[4] >= 0 {
			directive.expires = line[match[4]:match[5]]
		}
		out = append(out, directive)
	}
	return out
}

func parseInlineSuppressionLines(lines []string, hasIgnore bool, hasGosec bool) []inlineSuppression {
	out := make([]inlineSuppression, 0) //nolint:prealloc // directive count is unknown until each line is matched
	for idx, line := range lines {
		if hasIgnore && strings.Contains(line, "codeguard:ignore") {
			out = append(out, parseIgnoreDirectives(line, idx+1)...)
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
	directive, ok := index.matchOwn(sc, finding)
	if !ok {
		return inlineSuppression{}, false
	}
	for _, peer := range index.peers {
		if _, peerOK := peer.matchOwn(sc, finding); !peerOK {
			return inlineSuppression{}, false
		}
	}
	return directive, true
}

func (index *inlineSuppressionIndex) matchOwn(sc Context, finding core.Finding) (inlineSuppression, bool) {
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
	if !rules.MatchRulePattern(directive.ruleID, finding.RuleID) {
		return false
	}
	return !suppressionExpired(directive.expires, sc.Today)
}

// invalidCovering returns the invalid directives positioned where a valid
// codeguard:ignore would have covered finding: its own line, the line above,
// or anywhere in the file for a file-level finding.
func (index *inlineSuppressionIndex) invalidCovering(finding core.Finding) []inlineSuppression {
	var out []inlineSuppression
	for _, directive := range index.invalid {
		if finding.Line == 0 || directive.line == finding.Line || directive.line == finding.Line-1 {
			out = append(out, directive)
		}
	}
	return out
}
