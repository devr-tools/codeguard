package agentcontext

import (
	"regexp"
	"strings"
)

type refKind string

const (
	refPath      refKind = "path"
	refMake      refKind = "make"
	refNpmScript refKind = "npm-script"
)

// docReference is one resolvable claim a doc makes about the repository: a
// file/dir path, a make target, or an npm/pnpm/yarn script.
type docReference struct {
	kind    refKind
	value   string
	line    int
	display string
}

// extractOptions controls how much of a document is mined for references.
// Command fences (bash/sh/shell) are always parsed; commandFencesOnly limits
// extraction to them, which is the README rule's scope.
type extractOptions struct {
	commandFencesOnly bool
}

var (
	inlineCodePattern   = regexp.MustCompile("`([^`\n]+)`")
	markdownLinkPattern = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	lineSuffixPattern   = regexp.MustCompile(`:\d+$`)
)

// extractDocReferences walks a markdown (or plain-text) doc and returns the
// references that can be positively resolved against the repository. Fenced
// blocks other than shell command fences are skipped entirely: code samples,
// JSON, and captured output are full of tokens that merely look like paths.
func extractDocReferences(content string, opts extractOptions) []docReference {
	var refs []docReference
	seen := map[string]struct{}{}
	fence := fenceState{}
	for idx, line := range strings.Split(content, "\n") {
		lineNo := idx + 1
		if fence.observe(line) {
			continue
		}
		switch {
		case fence.inCommandFence():
			refs = appendRefs(refs, seen, lineNo, commandLineReferences(line, &fence))
		case fence.inFence() || opts.commandFencesOnly:
			continue
		default:
			refs = appendRefs(refs, seen, lineNo, proseLineReferences(line))
		}
	}
	return refs
}

func appendRefs(refs []docReference, seen map[string]struct{}, lineNo int, found []docReference) []docReference {
	for _, ref := range found {
		key := string(ref.kind) + "|" + ref.value
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		ref.line = lineNo
		refs = append(refs, ref)
	}
	return refs
}

// proseLineReferences extracts references from a line outside any fence:
// inline code spans (commands or paths), markdown link targets, and bare
// prose tokens that obviously denote files.
func proseLineReferences(line string) []docReference {
	var refs []docReference
	stripped := line
	for _, match := range inlineCodePattern.FindAllStringSubmatch(line, -1) {
		span := strings.TrimSpace(match[1])
		stripped = strings.Replace(stripped, match[0], " ", 1)
		if cmdRefs := commandStringReferences(span); len(cmdRefs) > 0 {
			refs = append(refs, cmdRefs...)
			continue
		}
		if value, ok := pathToken(span, false); ok {
			refs = append(refs, docReference{kind: refPath, value: value, display: span})
		}
	}
	for _, match := range markdownLinkPattern.FindAllStringSubmatch(stripped, -1) {
		if value, ok := pathToken(strings.TrimPrefix(match[1], "#"), false); ok {
			refs = append(refs, docReference{kind: refPath, value: value, display: match[1]})
		}
	}
	for _, token := range strings.Fields(markdownLinkPattern.ReplaceAllString(stripped, " ")) {
		if value, ok := pathToken(token, true); ok {
			refs = append(refs, docReference{kind: refPath, value: value, display: value})
		}
	}
	return refs
}

// pathToken decides whether a raw token is an obvious repo-relative path and
// normalizes it. requireExtension is set for bare prose tokens, which need a
// file extension to count; inline code and link targets may also be
// directories written with a trailing slash or a ./ prefix.
func pathToken(token string, requireExtension bool) (string, bool) {
	token = strings.TrimLeft(token, "\"'([{")
	token = strings.TrimRight(token, ".,:;!?)\"'}]")
	token = lineSuffixPattern.ReplaceAllString(token, "")
	if len(token) < 3 || !strings.Contains(token, "/") || hasUnresolvableRunes(token) {
		return "", false
	}
	if strings.HasPrefix(token, "/") || strings.HasPrefix(token, "~") || strings.Contains(token, "..") {
		return "", false
	}
	segments := strings.Split(strings.TrimPrefix(token, "./"), "/")
	// A dotted first segment that is not a dot-directory reads as a domain or
	// module path (github.com/..., example.io/...), never a repo file.
	if strings.Contains(segments[0], ".") && !strings.HasPrefix(segments[0], ".") {
		return "", false
	}
	last := segments[len(segments)-1]
	hasExtension := len(last) > 1 && strings.Contains(last[1:], ".")
	if requireExtension && !hasExtension {
		return "", false
	}
	if !hasExtension && !strings.HasSuffix(token, "/") && !strings.HasPrefix(token, "./") {
		return "", false
	}
	return strings.TrimPrefix(token, "./"), true
}

// hasUnresolvableRunes rejects tokens carrying URL schemes, globs, shell or
// template placeholders, version pins, or characters that never appear in the
// plain repo paths this rule is willing to judge.
func hasUnresolvableRunes(token string) bool {
	if strings.Contains(token, "://") {
		return true
	}
	for _, r := range token {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '/' || r == '-' || r == '+':
		default:
			return true
		}
	}
	return false
}
