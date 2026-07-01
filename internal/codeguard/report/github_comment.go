package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const githubCommentFindingLimit = 50

func writeGitHubComment(w io.Writer, report core.Report) error {
	if report.Summary.TotalFindings == 0 {
		return writeGitHubCommentClean(w, report)
	}
	return writeGitHubCommentFindings(w, report)
}

func writeGitHubCommentClean(w io.Writer, report core.Report) error {
	_, err := fmt.Fprintf(w, "## CodeGuard Fix Suggestions\n\nNo policy findings in this scan.\n\n%s\n", githubCommentSummary(report))
	return err
}

func writeGitHubCommentFindings(w io.Writer, report core.Report) error {
	if _, err := fmt.Fprintf(w, "## CodeGuard Fix Suggestions\n\n%s\n\n", githubCommentSummary(report)); err != nil {
		return err
	}

	written := 0
	for _, section := range report.Sections {
		if len(section.Findings) == 0 {
			continue
		}
		remainingSlots := githubCommentFindingLimit - written
		truncated, err := writeGitHubCommentSection(w, section.Name, section.Findings, remainingSlots)
		if err != nil {
			return err
		}
		written += min(len(section.Findings), remainingSlots)
		if truncated {
			return writeGitHubCommentTruncation(w, report.Summary.TotalFindings-written)
		}
	}
	return nil
}

func writeGitHubCommentSection(w io.Writer, name string, findings []core.Finding, limit int) (bool, error) {
	if _, err := fmt.Fprintf(w, "### %s\n\n", name); err != nil {
		return false, err
	}
	for idx, finding := range findings {
		if idx >= limit {
			return true, nil
		}
		if err := writeGitHubCommentFinding(w, idx+1, finding); err != nil {
			return false, err
		}
	}
	_, err := io.WriteString(w, "\n")
	return false, err
}

func writeGitHubCommentFinding(w io.Writer, index int, finding core.Finding) error {
	if _, err := fmt.Fprintf(w, "%d. `%s` at %s\n", index, finding.RuleID, githubCommentLocation(finding)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "   - Why: %s\n", githubCommentSentence(firstNonEmpty(finding.Why, finding.Message), "See report output for details.")); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "   - Fix: %s\n", githubCommentSentence(firstNonEmpty(finding.HowToFix), "See rule guidance."))
	return err
}

func writeGitHubCommentTruncation(w io.Writer, remaining int) error {
	if remaining <= 0 {
		return nil
	}
	_, err := fmt.Fprintf(w, "\n_Only the first %d findings are shown here. %d additional findings were omitted._\n", githubCommentFindingLimit, remaining)
	return err
}

func githubCommentSummary(report core.Report) string {
	return fmt.Sprintf(
		"Summary: %d pass, %d warn, %d fail, %d findings, %d suppressed.",
		report.Summary.PassedSections,
		report.Summary.WarnedSections,
		report.Summary.FailedSections,
		report.Summary.TotalFindings,
		report.Summary.SuppressedFindings,
	)
}

func githubCommentLocation(finding core.Finding) string {
	location := firstNonEmpty(strings.TrimSpace(finding.Path), "repository scope")
	if finding.Line > 0 {
		return fmt.Sprintf("`%s:%d`", location, finding.Line)
	}
	return fmt.Sprintf("`%s`", location)
}

func githubCommentSentence(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
