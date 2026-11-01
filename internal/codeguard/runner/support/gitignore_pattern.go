package support

import (
	"bufio"
	"bytes"
	"strings"
)

// gitignorePattern is one parsed line of a .gitignore (or info/exclude) file,
// following gitignore(5):
//
//   - a leading "!" negates the pattern (re-includes what an earlier pattern
//     excluded); "\!" is a literal "!".
//   - a trailing "/" restricts the pattern to directories.
//   - a "/" at the start or in the middle anchors the pattern to the directory
//     holding the ignore file; without one the pattern matches a name at any
//     depth below it.
//   - "*" and "?" never match "/", "[...]" is a bracket expression ("!" or "^"
//     negates it), and "\" escapes the next character.
//   - "**" as a whole segment matches any number of directories: a leading
//     "**/" matches in every directory, "a/**/b" matches zero or more
//     directories between a and b, and a trailing "/**" matches everything
//     inside. Any other run of asterisks is an ordinary "*".
//
// Matching is case-sensitive regardless of the host's core.ignorecase, so a
// scan reports the same files on a case-insensitive macOS checkout as on a
// Linux CI runner.
type gitignorePattern struct {
	segments []string
	negate   bool
	dirOnly  bool
	anchored bool
}

// parseGitignore parses the contents of one ignore file. Lines that are blank,
// comments, or reduce to nothing are skipped.
func parseGitignore(data []byte) []gitignorePattern {
	var patterns []gitignorePattern
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 4096), maxGitignoreFileBytes+1)
	for scanner.Scan() {
		if pattern, ok := parseGitignoreLine(scanner.Text()); ok {
			patterns = append(patterns, pattern)
		}
	}
	return patterns
}

func parseGitignoreLine(line string) (gitignorePattern, bool) {
	line = strings.TrimSuffix(line, "\r")
	if line == "" || strings.HasPrefix(line, "#") {
		return gitignorePattern{}, false
	}
	line = trimUnescapedTrailingSpaces(line)
	var pattern gitignorePattern
	if strings.HasPrefix(line, "!") {
		pattern.negate = true
		line = line[1:]
	}
	if strings.HasSuffix(line, "/") && !strings.HasSuffix(line, `\/`) {
		pattern.dirOnly = true
		line = strings.TrimRight(line, "/")
	}
	if strings.HasPrefix(line, "/") {
		pattern.anchored = true
		line = strings.TrimLeft(line, "/")
	}
	if line == "" {
		return gitignorePattern{}, false
	}
	if strings.Contains(line, "/") {
		pattern.anchored = true
	}
	segments := strings.Split(line, "/")
	pattern.segments = segments[:0]
	for _, segment := range segments {
		if segment == "" {
			continue // "a//b" is "a/b"
		}
		if segment == "**" && len(pattern.segments) > 0 && pattern.segments[len(pattern.segments)-1] == "**" {
			continue // consecutive "**" segments are one
		}
		pattern.segments = append(pattern.segments, segment)
	}
	if len(pattern.segments) == 0 {
		return gitignorePattern{}, false
	}
	return pattern, true
}

// trimUnescapedTrailingSpaces drops trailing spaces unless the last one is
// escaped with a backslash ("foo\ " keeps one trailing space).
func trimUnescapedTrailingSpaces(line string) string {
	end := len(line)
	for end > 0 && line[end-1] == ' ' {
		if end >= 2 && line[end-2] == '\\' {
			break
		}
		end--
	}
	return line[:end]
}

// matches reports whether rel (slash-separated, relative to the directory that
// holds the ignore file) matches the pattern. isDir says whether rel names a
// directory.
func (p gitignorePattern) matches(rel string, isDir bool) bool {
	if p.dirOnly && !isDir {
		return false
	}
	if !p.anchored {
		name := rel
		if idx := strings.LastIndexByte(rel, '/'); idx >= 0 {
			name = rel[idx+1:]
		}
		return wildmatchSegment(p.segments[0], name)
	}
	return matchGitignoreSegments(p.segments, strings.Split(rel, "/"))
}

func matchGitignoreSegments(pattern []string, path []string) bool {
	if len(pattern) == 0 {
		return len(path) == 0
	}
	if pattern[0] == "**" {
		if len(pattern) == 1 {
			// A trailing "/**" matches everything inside, but not the
			// directory itself.
			return len(path) > 0
		}
		for skip := 0; skip <= len(path); skip++ {
			if matchGitignoreSegments(pattern[1:], path[skip:]) {
				return true
			}
		}
		return false
	}
	if len(path) == 0 || !wildmatchSegment(pattern[0], path[0]) {
		return false
	}
	return matchGitignoreSegments(pattern[1:], path[1:])
}

// wildmatchSegment matches one path segment (no "/") against one pattern
// segment with "*", "?", "[...]" and backslash escapes. It backtracks only to
// the most recent "*", which is sufficient because "*" cannot cross a "/" and
// the input is a single segment, so the worst case is linear in
// len(pattern)*len(name).
func wildmatchSegment(pattern string, name string) bool {
	px, nx := 0, 0
	starPx, starNx := -1, -1
	for px < len(pattern) || nx < len(name) {
		if px < len(pattern) {
			if advance, ok := wildmatchStep(pattern, px, name, nx); ok {
				if advance < 0 {
					starPx, starNx = px, nx
					px++
					continue
				}
				px += advance
				nx++
				continue
			}
		}
		if starPx >= 0 && starNx < len(name) {
			starNx++
			px, nx = starPx+1, starNx
			continue
		}
		return false
	}
	return true
}

// wildmatchStep tries to consume pattern[px:] against name[nx]. It returns the
// number of pattern bytes consumed (the name always advances by one byte), or
// -1 for a "*", which the caller records as a backtrack point.
func wildmatchStep(pattern string, px int, name string, nx int) (int, bool) {
	switch c := pattern[px]; c {
	case '*':
		return -1, true
	case '?':
		return 1, nx < len(name)
	case '[':
		if nx >= len(name) {
			return 0, false
		}
		matched, width, valid := matchBracket(pattern[px:], name[nx])
		if valid {
			return width, matched
		}
		return 1, name[nx] == '['
	case '\\':
		if px+1 < len(pattern) {
			return 2, nx < len(name) && name[nx] == pattern[px+1]
		}
		return 1, nx < len(name) && name[nx] == '\\'
	default:
		return 1, nx < len(name) && name[nx] == c
	}
}

// matchBracket evaluates the bracket expression at the start of pattern
// against ch. It returns whether ch matched, how many pattern bytes the
// expression spans, and whether the expression was well formed (an unclosed
// "[" is a literal).
func matchBracket(pattern string, ch byte) (bool, int, bool) {
	i := 1
	negate := false
	if i < len(pattern) && (pattern[i] == '!' || pattern[i] == '^') {
		negate = true
		i++
	}
	matched := false
	first := true
	for i < len(pattern) {
		c := pattern[i]
		if c == ']' && !first {
			return matched != negate, i + 1, true
		}
		first = false
		if c == '[' && i+1 < len(pattern) && pattern[i+1] == ':' {
			if end := strings.Index(pattern[i+2:], ":]"); end >= 0 {
				if matchCharClass(pattern[i+2:i+2+end], ch) {
					matched = true
				}
				i += end + 4
				continue
			}
		}
		lo, width := bracketChar(pattern, i)
		i += width
		if i+1 < len(pattern) && pattern[i] == '-' && pattern[i+1] != ']' {
			hi, hiWidth := bracketChar(pattern, i+1)
			i += 1 + hiWidth
			if lo <= ch && ch <= hi {
				matched = true
			}
			continue
		}
		if ch == lo {
			matched = true
		}
	}
	return false, 0, false
}

func bracketChar(pattern string, i int) (byte, int) {
	if pattern[i] == '\\' && i+1 < len(pattern) {
		return pattern[i+1], 2
	}
	return pattern[i], 1
}

// posixCharClasses implements the bracket-expression character classes
// ([:alpha:] and friends) over ASCII.
var posixCharClasses = map[string]func(ch byte) bool{
	"alnum":  func(ch byte) bool { return isASCIILetter(ch) || isASCIIDigit(ch) },
	"alpha":  isASCIILetter,
	"digit":  isASCIIDigit,
	"lower":  func(ch byte) bool { return 'a' <= ch && ch <= 'z' },
	"upper":  func(ch byte) bool { return 'A' <= ch && ch <= 'Z' },
	"space":  func(ch byte) bool { return ch == ' ' || ('\t' <= ch && ch <= '\r') },
	"blank":  func(ch byte) bool { return ch == ' ' || ch == '\t' },
	"xdigit": func(ch byte) bool { return isASCIIDigit(ch) || ('a' <= ch && ch <= 'f') || ('A' <= ch && ch <= 'F') },
	"punct":  func(ch byte) bool { return ch > ' ' && ch < 0x7f && !isASCIILetter(ch) && !isASCIIDigit(ch) },
	"print":  func(ch byte) bool { return ch >= ' ' && ch < 0x7f },
	"graph":  func(ch byte) bool { return ch > ' ' && ch < 0x7f },
	"cntrl":  func(ch byte) bool { return ch < ' ' || ch == 0x7f },
}

func matchCharClass(class string, ch byte) bool {
	matches, ok := posixCharClasses[class]
	return ok && matches(ch)
}

func isASCIILetter(ch byte) bool { return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') }
func isASCIIDigit(ch byte) bool  { return '0' <= ch && ch <= '9' }
