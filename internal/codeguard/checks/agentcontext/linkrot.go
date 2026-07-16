package agentcontext

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// linkSchemePattern recognizes URI schemes (https:, mailto:, vscode:) before
// any path separator; such targets are external and never checked — this rule
// does no network I/O.
var linkSchemePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// markdownLink is one link target found outside code fences, with the line it
// appears on.
type markdownLink struct {
	target string
	line   int
}

// docLinkRotFindings checks markdown link targets in the agent docs and the
// root README against the repository, reporting links that resolve to
// nothing. Findings per doc share the drift rules' cap.
func docLinkRotFindings(env support.Context, resolver *repoResolver, agentDocs []string) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, rel := range append(append([]string{}, agentDocs...), "README.md") {
		data, ok := readCappedDocFile(resolver.root, rel)
		if !ok {
			continue
		}
		findings = append(findings, linkRotFindingsForDoc(env, resolver, rel, string(data))...)
	}
	return findings
}

func linkRotFindingsForDoc(env support.Context, resolver *repoResolver, docRel string, content string) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, link := range extractMarkdownLinks(content) {
		if len(findings) >= maxDriftFindingsPerDoc {
			break
		}
		target, ok := linkPathToCheck(link.target)
		if !ok || linkResolves(resolver, docRel, target) {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID: "context.doc-link-rot",
			Level:  "warn",
			Path:   docRel,
			Line:   link.line,
			Column: 1,
			Message: fmt.Sprintf("%s links to %q, which does not resolve to a file or directory in the repository; "+
				"broken links send agents and readers to dead ends — fix the link target or remove the link", docRel, link.target),
		}))
	}
	return findings
}

// extractMarkdownLinks collects markdown link targets outside code fences,
// deduplicated per document. Fenced blocks are code samples, where link
// syntax is content, not navigation.
func extractMarkdownLinks(content string) []markdownLink {
	var links []markdownLink
	seen := map[string]struct{}{}
	fence := fenceState{}
	for idx, line := range strings.Split(content, "\n") {
		if fence.observe(line) || fence.inFence() {
			continue
		}
		for _, match := range markdownLinkPattern.FindAllStringSubmatch(line, -1) {
			target := match[1]
			if _, dup := seen[target]; dup {
				continue
			}
			seen[target] = struct{}{}
			links = append(links, markdownLink{target: target, line: idx + 1})
		}
	}
	return links
}

// linkPathToCheck reduces a raw link target to the repository path this rule
// is willing to judge, preserving precision over recall:
//   - external URLs (any scheme) and protocol-relative //host links: skipped
//   - pure #anchor links: skipped; path.md#anchor checks only the path part
//   - editor-style :line suffixes are stripped before resolution
//   - .. traversals, query strings, and templated or placeholder targets
//     (<name>, $VAR, %20, globs): skipped — they cannot be proven broken
func linkPathToCheck(target string) (string, bool) {
	if idx := strings.IndexByte(target, '#'); idx >= 0 {
		target = target[:idx]
	}
	target = lineSuffixPattern.ReplaceAllString(target, "")
	if target == "" || strings.HasPrefix(target, "//") || linkSchemePattern.MatchString(target) {
		return "", false
	}
	if strings.Contains(target, "..") || strings.Contains(target, "?") || hasUnresolvableRunes(target) {
		return "", false
	}
	return target, true
}

// linkResolves reports whether a checkable link target exists. Relative
// targets resolve against the doc's own directory (markdown semantics) and
// the repo root (the common shorthand); absolute /path targets resolve
// against the repo root only, the convention hosted viewers apply — an
// absolute filesystem path baked into a doc resolves nowhere and is exactly
// the rot this rule exists to catch.
func linkResolves(resolver *repoResolver, docRel string, target string) bool {
	if strings.HasPrefix(target, "/") {
		return resolver.pathExists(strings.TrimPrefix(target, "/"))
	}
	if resolver.pathExists(target) {
		return true
	}
	docDir := path.Dir(docRel)
	return docDir != "." && resolver.pathExists(path.Join(docDir, target))
}
