package githubaction

import (
	"sort"
	"strings"
)

// DefaultReviewCommentLimit caps inline comments per review when the caller
// does not set a limit.
const DefaultReviewCommentLimit = 25

const sideRight = "RIGHT"

// plannedComment pairs an inline comment with the finding it reports, so a
// rejected comment can still be listed in the review body.
type plannedComment struct {
	comment ReviewComment
	finding Finding
}

type reviewPlan struct {
	comments    []plannedComment
	overflow    []Finding
	duplicates  int
	outsideDiff int
}

// ReportFindings returns the unsuppressed fail and warn findings of a scan
// report that point at a file line, sorted fail-first and then by location so
// the comment cap keeps the most severe findings.
func ReportFindings(report Report) []Finding {
	var findings []Finding
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			finding.Level = strings.ToLower(strings.TrimSpace(finding.Level))
			if finding.Suppressed || finding.Path == "" || finding.Line <= 0 {
				continue
			}
			if finding.Level == levelFail || finding.Level == levelWarn {
				findings = append(findings, finding)
			}
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		if (left.Level == levelFail) != (right.Level == levelFail) {
			return left.Level == levelFail
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.RuleID < right.RuleID
	})
	return findings
}

// planReview decides, for each finding, whether it becomes an inline comment,
// is skipped (outside the diff or already commented), or overflows the limit.
func planReview(findings []Finding, diff DiffLines, existing map[string]bool, limit int, suggest SuggestionFunc) reviewPlan {
	var plan reviewPlan
	seen := map[string]bool{}
	for _, finding := range findings {
		if !diff.Covers(finding.Path, finding.Line, finding.Line) {
			plan.outsideDiff++
			continue
		}
		fingerprint := FindingFingerprint(finding)
		if existing[fingerprint] || seen[fingerprint] {
			plan.duplicates++
			continue
		}
		seen[fingerprint] = true
		if len(plan.comments) >= limit {
			plan.overflow = append(plan.overflow, finding)
			continue
		}
		plan.comments = append(plan.comments, plannedComment{comment: buildReviewComment(finding, diff, suggest), finding: finding})
	}
	return plan
}

// buildReviewComment anchors a comment on the finding's line, widening it to a
// multi-line comment when an exact suggestion replaces several lines.
func buildReviewComment(finding Finding, diff DiffLines, suggest SuggestionFunc) ReviewComment {
	comment := ReviewComment{Path: finding.Path, Line: finding.Line, Side: sideRight}
	suggestion, ok := exactSuggestion(finding, diff, suggest)
	if !ok {
		comment.Body = RenderReviewComment(finding, nil)
		return comment
	}
	comment.Line = suggestion.EndLine
	if suggestion.StartLine < suggestion.EndLine {
		comment.StartLine = suggestion.StartLine
		comment.StartSide = sideRight
	}
	comment.Body = RenderReviewComment(finding, &suggestion)
	return comment
}

// exactSuggestion accepts a suggestion only when it covers the finding's line
// and every replaced line is in one diff hunk; otherwise GitHub would reject
// the comment or the suggestion would replace the wrong lines.
func exactSuggestion(finding Finding, diff DiffLines, suggest SuggestionFunc) (Suggestion, bool) {
	if suggest == nil || !suggestionAllowed(finding) {
		return Suggestion{}, false
	}
	suggestion, ok := suggest(finding, diff)
	if !ok {
		return Suggestion{}, false
	}
	if suggestion.StartLine > finding.Line || suggestion.EndLine < finding.Line {
		return Suggestion{}, false
	}
	if !diff.Covers(finding.Path, suggestion.StartLine, suggestion.EndLine) {
		return Suggestion{}, false
	}
	return suggestion, true
}
