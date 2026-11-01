package contextgen

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrNoMarkers reports a file with no codeguard-managed blocks. Update never
// rewrites such a file: the caller shows a proposal and asks a human to add
// markers.
var ErrNoMarkers = errors.New("no codeguard:begin/end markers found")

// UpdateResult is the outcome of refreshing managed blocks in place.
type UpdateResult struct {
	// Content is the refreshed file. Every byte outside the managed blocks'
	// bodies is identical to the input.
	Content string
	// Changed lists managed blocks whose body was rewritten.
	Changed []string
	// Unchanged lists managed blocks whose body was already current.
	Unchanged []string
	// Unknown lists marker names codeguard does not manage; their bodies are
	// left untouched.
	Unknown []string
	// Missing lists managed blocks the file has no markers for; they are not
	// added.
	Missing []string
}

var markerLine = regexp.MustCompile(`^[ \t]*<!--[ \t]*codeguard:(begin|end)[ \t]+([A-Za-z0-9_-]+)[ \t]*-->[ \t]*\r?$`)

// markedBlock is one begin/end pair: bodyStart is the byte offset just after
// the begin marker's line, bodyEnd the offset where the end marker's line
// starts.
type markedBlock struct {
	name      string
	bodyStart int
	bodyEnd   int
	crlf      bool
}

// parseBlocks finds every begin/end pair. Malformed nesting is an error, not
// something to repair: guessing where a block ends could eat human text.
func parseBlocks(doc string) ([]markedBlock, error) {
	var blocks []markedBlock
	var open *markedBlock
	seen := map[string]bool{}
	offset := 0
	for lineNo, line := range strings.SplitAfter(doc, "\n") {
		lineStart := offset
		offset += len(line)
		match := markerLine.FindStringSubmatch(strings.TrimSuffix(line, "\n"))
		if match == nil {
			continue
		}
		kind, name := match[1], match[2]
		switch {
		case kind == "begin" && open != nil:
			return nil, fmt.Errorf("line %d: codeguard:begin %s opened inside block %q", lineNo+1, name, open.name)
		case kind == "begin" && seen[name]:
			return nil, fmt.Errorf("line %d: duplicate codeguard block %q", lineNo+1, name)
		case kind == "begin":
			seen[name] = true
			open = &markedBlock{name: name, bodyStart: offset, crlf: strings.HasSuffix(line, "\r\n")}
		case open == nil || open.name != name:
			return nil, fmt.Errorf("line %d: codeguard:end %s has no matching begin", lineNo+1, name)
		default:
			open.bodyEnd = lineStart
			blocks = append(blocks, *open)
			open = nil
		}
	}
	if open != nil {
		return nil, fmt.Errorf("codeguard:begin %s is never closed", open.name)
	}
	return blocks, nil
}

// Update refreshes the bodies of the managed blocks present in existing with
// freshly rendered content from facts. It returns ErrNoMarkers when existing
// has no managed blocks at all, and a parse error for malformed markers.
func Update(existing string, facts Facts) (UpdateResult, error) {
	blocks, err := parseBlocks(existing)
	if err != nil {
		return UpdateResult{}, err
	}
	if len(blocks) == 0 {
		return UpdateResult{}, ErrNoMarkers
	}
	known := map[string]managedBlock{}
	for _, block := range managedBlocks {
		known[block.name] = block
	}
	result := UpdateResult{}
	present := map[string]bool{}
	var b strings.Builder
	cursor := 0
	for _, block := range blocks {
		present[block.name] = true
		def, ok := known[block.name]
		if !ok {
			result.Unknown = append(result.Unknown, block.name)
			continue
		}
		fresh := blockBody(def, facts)
		if block.crlf {
			fresh = strings.ReplaceAll(fresh, "\n", "\r\n")
		}
		if existing[block.bodyStart:block.bodyEnd] == fresh {
			result.Unchanged = append(result.Unchanged, block.name)
			continue
		}
		b.WriteString(existing[cursor:block.bodyStart])
		b.WriteString(fresh)
		cursor = block.bodyEnd
		result.Changed = append(result.Changed, block.name)
	}
	b.WriteString(existing[cursor:])
	result.Content = b.String()
	for _, block := range managedBlocks {
		if !present[block.name] {
			result.Missing = append(result.Missing, block.name)
		}
	}
	return result, nil
}

// Proposal is what `context update` would make of a file without markers:
// the file unchanged, followed by every managed section. It is shown as a
// diff and never written.
func Proposal(existing string, facts Facts) string {
	sep := ""
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		sep = "\n"
	}
	return existing + sep + RenderSections(facts)
}
