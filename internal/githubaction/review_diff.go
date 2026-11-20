package githubaction

import (
	"strconv"
	"strings"
)

// DiffLines maps a repository-relative path to the RIGHT-side lines of the
// pull request diff, keyed by new-file line number, with each line's text.
// GitHub accepts a review comment only on a line in a diff hunk, so a finding
// whose line is absent here is never sent as an inline comment.
type DiffLines map[string]map[int]string

// ParsePatch returns the commentable RIGHT-side lines of one file's unified
// diff patch (the "patch" field of GET /pulls/{n}/files): added lines and
// context lines, keyed by their new-file line number. Deleted lines exist only
// on the LEFT side and are skipped.
func ParsePatch(patch string) map[int]string {
	lines := map[int]string{}
	next := 0
	inHunk := false
	for _, raw := range strings.Split(patch, "\n") {
		raw = strings.TrimSuffix(raw, "\r")
		if strings.HasPrefix(raw, "@@") {
			start, ok := hunkNewStart(raw)
			next, inHunk = start, ok
			continue
		}
		if !inHunk || raw == "" {
			continue
		}
		switch raw[0] {
		case '+', ' ':
			lines[next] = raw[1:]
			next++
		case '-', '\\':
			// A deletion has no new-file line; "\ No newline at end of file"
			// annotates the previous line.
		default:
			inHunk = false
		}
	}
	return lines
}

// hunkNewStart parses the new-file start line from "@@ -a,b +c,d @@".
func hunkNewStart(header string) (int, bool) {
	fields := strings.Fields(header)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "+") {
		return 0, false
	}
	startText, _, _ := strings.Cut(strings.TrimPrefix(fields[2], "+"), ",")
	start, err := strconv.Atoi(startText)
	if err != nil || start < 0 {
		return 0, false
	}
	return start, true
}

// Text returns the diff text of a RIGHT-side line.
func (d DiffLines) Text(path string, line int) (string, bool) {
	text, ok := d[path][line]
	return text, ok
}

// Covers reports whether every line from start to end is a RIGHT-side diff
// line of path. Consecutive new-file lines that are all present belong to one
// hunk, which GitHub requires of a multi-line comment.
func (d DiffLines) Covers(path string, start, end int) bool {
	if start <= 0 || end < start {
		return false
	}
	file, ok := d[path]
	if !ok {
		return false
	}
	for line := start; line <= end; line++ {
		if _, ok := file[line]; !ok {
			return false
		}
	}
	return true
}
