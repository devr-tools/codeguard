// Package rulepacktest runs custom rule packs over annotated fixture files
// and compares the findings with the annotations, in the style of
// `semgrep --test`.
package rulepacktest

import (
	"regexp"
	"strings"
)

// Annotation kinds. A "ruleid" annotation expects the rule to report on its
// target line; an "ok" annotation expects it not to.
const (
	KindRuleID = "ruleid"
	KindOK     = "ok"
)

// Annotation is one expectation parsed from a fixture comment.
type Annotation struct {
	Kind   string
	RuleID string
	// Line is the line the expectation is about: the next line that is not
	// itself an annotation, or 0 for a file-level annotation (ruleid-file,
	// ok-file), which matches the file-level findings path-only rules report.
	Line int
	// SourceLine is the line the annotation comment sits on.
	SourceLine int
}

// annotationPattern matches a comment-only line holding an annotation, in any
// common comment syntax: //, #, --, ;, %, ', /*, *, <!--, {#, and REM.
var annotationPattern = regexp.MustCompile(`^\s*(?://+|#+|--|;+|%+|'|/\*+|\*|<!--|\{#|(?i:rem)\b)\s*(ruleid|ok)(-file)?\s*:\s*([A-Za-z0-9_.\-]+(?:\s*,\s*[A-Za-z0-9_.\-]+)*)`)

// ParseAnnotations returns the annotations in a fixture's content. A line
// may list several rule ids separated by commas, and consecutive annotation
// lines all apply to the first non-annotation line after them.
func ParseAnnotations(content string) []Annotation {
	lines := strings.Split(content, "\n")
	var out []Annotation
	var pending []Annotation
	for idx, line := range lines {
		lineNo := idx + 1
		match := annotationPattern.FindStringSubmatch(line)
		if match == nil {
			for i := range pending {
				pending[i].Line = lineNo
			}
			out = append(out, pending...)
			pending = nil
			continue
		}
		for _, ruleID := range strings.Split(match[3], ",") {
			annotation := Annotation{Kind: match[1], RuleID: strings.TrimSpace(ruleID), SourceLine: lineNo}
			if match[2] != "" {
				out = append(out, annotation)
				continue
			}
			pending = append(pending, annotation)
		}
	}
	// Annotations at the end of the file have no line to describe; they keep
	// Line -1 so they can never be satisfied and the mismatch is visible.
	for i := range pending {
		pending[i].Line = -1
	}
	return append(out, pending...)
}
