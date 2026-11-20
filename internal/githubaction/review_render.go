package githubaction

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/devr-tools/codeguard/internal/codeguard/rules/ruledocs"
)

// ReviewMarkerPrefix starts the hidden first line of every inline review
// comment; the rest of the line carries the finding fingerprint.
const ReviewMarkerPrefix = "<!-- codeguard-review:"

// ReviewBodyMarker opens the body of the review that carries the comments.
const ReviewBodyMarker = "<!-- codeguard-review -->"

const (
	maxMessageRunes  = 1000
	maxHowToFixRunes = 600
)

var (
	fingerprintPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
	codeSpanUnsafe     = regexp.MustCompile(`[^A-Za-z0-9._/:-]`)
	markdownEscaper    = strings.NewReplacer(
		`\`, `\\`, "`", "\\`", `*`, `\*`, `_`, `\_`, `{`, `\{`, `}`, `\}`,
		`[`, `\[`, `]`, `\]`, `(`, `\(`, `)`, `\)`, `#`, `\#`, `+`, `\+`,
		`-`, `\-`, `!`, `\!`, `|`, `\|`, `~`, `\~`,
		`&`, "&amp;", `<`, "&lt;", `>`, "&gt;",
		// A zero-width space after @ keeps a message from pinging a user or team.
		`@`, "@\u200b",
	)
)

// FindingFingerprint is the dedupe key carried in a comment's marker. It
// prefers the context fingerprint, which survives edits that only shift the
// finding's line, and is always a short token that cannot end the HTML
// comment it sits in.
func FindingFingerprint(finding Finding) string {
	for _, candidate := range []string{finding.ContextFingerprint, finding.Fingerprint} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if fingerprintPattern.MatchString(candidate) {
			return candidate
		}
		return hashToken(candidate)
	}
	return hashToken(fmt.Sprintf("%s\x00%s\x00%d", finding.RuleID, finding.Path, finding.Line))
}

func hashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func reviewMarker(fingerprint string) string {
	return ReviewMarkerPrefix + fingerprint + " -->"
}

// reviewFingerprintFromBody returns the fingerprint in a comment's first-line
// marker. Text later in the body is ignored, so a quoted marker inside a
// finding message cannot claim another finding.
func reviewFingerprintFromBody(body string) (string, bool) {
	firstLine, _, _ := strings.Cut(body, "\n")
	firstLine = strings.TrimSpace(firstLine)
	if !strings.HasPrefix(firstLine, ReviewMarkerPrefix) || !strings.HasSuffix(firstLine, "-->") {
		return "", false
	}
	fingerprint := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(firstLine, ReviewMarkerPrefix), "-->"))
	if !fingerprintPattern.MatchString(fingerprint) {
		return "", false
	}
	return fingerprint, true
}

// EscapeMarkdown renders repo-controlled text as one inert markdown line:
// line breaks become spaces, markdown and HTML metacharacters are escaped,
// mentions are defused, and the result is capped at maxRunes.
func EscapeMarkdown(text string, maxRunes int) string {
	text = strings.Join(strings.Fields(text), " ")
	if maxRunes > 0 && utf8.RuneCountInString(text) > maxRunes {
		text = string([]rune(text)[:maxRunes]) + "…"
	}
	return markdownEscaper.Replace(text)
}

// codeSpan wraps an identifier such as a rule id in backticks after removing
// every character that could close the span or start markup.
func codeSpan(text string) string {
	cleaned := codeSpanUnsafe.ReplaceAllString(text, "")
	if cleaned == "" {
		cleaned = "unknown"
	}
	return "`" + cleaned + "`"
}

// SuggestionBlock renders replacement lines as a GitHub suggestion fence. The
// fence is longer than any backtick run in the replacement, so a line holding
// ``` cannot close it early.
func SuggestionBlock(lines []string) string {
	longest := 0
	for _, line := range lines {
		run := 0
		for _, r := range line {
			if r != '`' {
				run = 0
				continue
			}
			run++
			longest = max(longest, run)
		}
	}
	fence := strings.Repeat("`", max(3, longest+1))
	var b strings.Builder
	b.WriteString(fence + "suggestion\n")
	for _, line := range lines {
		b.WriteString(strings.NewReplacer("\r", "", "\n", "").Replace(line) + "\n")
	}
	b.WriteString(fence + "\n")
	return b.String()
}

// RenderReviewComment builds the body of one inline comment. suggestion is nil
// unless the finding has an exact replacement.
func RenderReviewComment(finding Finding, suggestion *Suggestion) string {
	var b strings.Builder
	b.WriteString(reviewMarker(FindingFingerprint(finding)) + "\n")
	fmt.Fprintf(&b, "**CodeGuard** %s (%s)\n\n", codeSpan(finding.RuleID), codeSpan(finding.Level))
	b.WriteString(EscapeMarkdown(finding.Message, maxMessageRunes) + "\n")
	if howTo := strings.TrimSpace(finding.HowToFix); howTo != "" {
		b.WriteString("\n**How to fix:** " + EscapeMarkdown(howTo, maxHowToFixRunes) + "\n")
	}
	if suggestion != nil {
		b.WriteString("\n" + SuggestionBlock(suggestion.Lines))
	}
	if strings.TrimSpace(finding.RuleID) != "" {
		fmt.Fprintf(&b, "\n[Rule docs](%s)\n", ruledocs.RuleURL(codeSpanUnsafe.ReplaceAllString(finding.RuleID, "")))
	}
	return b.String()
}

// renderReviewBody builds the review summary: how many inline comments were
// posted plus the findings that could not be placed inline.
func renderReviewBody(posted int, limit int, overflow []Finding, rejected []Finding) string {
	var b strings.Builder
	b.WriteString(ReviewBodyMarker + "\n")
	fmt.Fprintf(&b, "**CodeGuard** posted %d inline comment(s) on changed lines.\n", posted)
	writeFindingList(&b, fmt.Sprintf("%d more finding(s) on changed lines exceeded the review comment limit of %d:", len(overflow), limit), overflow)
	writeFindingList(&b, fmt.Sprintf("GitHub rejected %d inline comment(s); the findings are listed here instead:", len(rejected)), rejected)
	return TruncateCommentBody(b.String())
}

func writeFindingList(b *strings.Builder, heading string, findings []Finding) {
	if len(findings) == 0 {
		return
	}
	b.WriteString("\n" + heading + "\n\n")
	for _, finding := range findings {
		fmt.Fprintf(b, "- %s:%d %s %s\n", EscapeMarkdown(finding.Path, 300), finding.Line, codeSpan(finding.RuleID), EscapeMarkdown(finding.Message, 300))
	}
}
