package prompts

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	// bypassFlagPatterns are the CLI switches and modes that turn off an
	// agent's approval prompts or sandbox (Claude Code, Codex, Gemini CLI,
	// Copilot CLI). Naming one is only dangerous when it is not prohibited,
	// so these are negation-sensitive.
	bypassFlagPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)--(?:allow-)?dangerously-skip-permissions\b`),
		regexp.MustCompile(`(?i)--dangerously-bypass-approvals-and-sandbox\b`),
		regexp.MustCompile(`(?i)(?:^|[^\w-])--yolo\b`),
		regexp.MustCompile(`(?i)--approval-mode[= ]+yolo\b`),
		regexp.MustCompile(`(?i)--allow-all-tools\b`),
		regexp.MustCompile(`(?i)--trust-all-tools\b`),
		regexp.MustCompile(`(?i)(?:--ask-for-approval|(?:^|\s)-a)[= ]+never\b`),
		regexp.MustCompile(`(?i)\bdanger-full-access\b`),
		regexp.MustCompile(`\bbypassPermissions\b`),
	}
	// bypassPhrasePatterns describe disabling a safeguard in prose. They are
	// negation-sensitive too: "never bypass the sandbox" is the opposite of
	// a bypass instruction.
	bypassPhrasePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(skip|disable|bypass|ignore|override|turn off)\b.{0,40}\b(approvals?|sandbox(ing)?|polic(y|ies)|guardrails?|safety|permissions?|confirmations?)\b`),
		regexp.MustCompile(`(?i)\bwithout\s+(asking|requesting|waiting|prompting|checking)\b.{0,30}\b(approval|confirmation|permission|consent)\b`),
		regexp.MustCompile(`(?i)\balways comply\b.{0,40}\bwithout\b.{0,20}\b(approval|confirmation)\b`),
		regexp.MustCompile(`(?i)\bauto-?approve\s+(all|every(thing)?|any)\b`),
		regexp.MustCompile(`(?i)\byolo mode\b`),
	}
	// noConfirmationPatterns carry their own negation: the instruction is to
	// stop asking. A preceding "never"/"don't" is the danger, not a
	// prohibition, so these are not negation-sensitive.
	noConfirmationPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(never|don'?t|do not|no need to|you don'?t need to|stop)\s+(ask|request|wait for|prompt|check with)\b.{0,25}\b(approval|confirmation|permission|consent)\b`),
		regexp.MustCompile(`(?i)\bnever\s+(seek|require|get)\b.{0,25}\b(approval|confirmation|permission|consent)\b`),
	}
	// prohibitionCue recognizes language that forbids what follows or
	// precedes it within one sentence.
	prohibitionCue = regexp.MustCompile(`(?i)\b(never|not|don'?t|dont|cannot|can'?t|mustn'?t|shouldn'?t|avoid|forbid(den)?|prohibit(ed)?|disallow(ed)?|refuse|banned|instead of)\b`)
	// prohibitionHeading recognizes a heading or list intro ("## Never do
	// this", "Do not:") under which listed commands are anti-examples.
	prohibitionHeading = regexp.MustCompile(`(?i)\b(never|don'?t|do not|avoid|forbidden|prohibited|anti-?patterns?|not allowed|must not|banned)\b`)
	listItemLine       = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+`)
	sentenceBreak      = regexp.MustCompile(`[.!?;](?:\s|$)`)
	markdownHeading    = regexp.MustCompile(`^\s{0,3}#{1,6}\s`)
)

// dangerousInstructionFindings scans an agent instruction document line by
// line. It tracks whether the current line sits under a prohibition heading
// or list intro so that a "never do this" section that names a bypass flag
// is not reported as an instruction to use it.
func dangerousInstructionFindings(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	prohibited := false
	listIntro := false
	for idx, line := range splitLines(data) {
		trimmed := strings.TrimSpace(line)
		switch {
		case markdownHeading.MatchString(line):
			prohibited = prohibitionHeading.MatchString(line)
			listIntro = false
		case strings.HasSuffix(trimmed, ":") && !listItemLine.MatchString(line):
			listIntro = prohibitionHeading.MatchString(trimmed)
		case trimmed != "" && !listItemLine.MatchString(line):
			listIntro = false
		}
		loc, confidence, found := dangerousInstruction(line, prohibited || listIntro)
		if !found {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID: "prompts.agent-dangerous-instructions",
			Level:  "fail",
			Path:   file,
			Line:   idx + 1,
			Column: loc[0] + 1,
			Message: fmt.Sprintf("agent instructions tell the agent to bypass approvals, permissions, or its sandbox (%q); keep approval and sandbox checks on and grant scoped permissions instead",
				strings.TrimSpace(line[loc[0]:loc[1]])),
			Confidence: confidence,
		}))
	}
	return findings
}

// dangerousInstruction returns the span of the strongest bypass signal on
// line: an unprohibited bypass flag first, then "stop asking" phrasing, then
// bypass prose.
func dangerousInstruction(line string, underProhibition bool) ([]int, string, bool) {
	if !underProhibition {
		if loc := firstUnnegatedMatch(line, bypassFlagPatterns); loc != nil {
			return loc, core.ConfidenceHigh, true
		}
	}
	for _, pattern := range noConfirmationPatterns {
		if loc := pattern.FindStringIndex(line); loc != nil {
			return loc, core.ConfidenceMedium, true
		}
	}
	if !underProhibition {
		if loc := firstUnnegatedMatch(line, bypassPhrasePatterns); loc != nil {
			return loc, core.ConfidenceMedium, true
		}
	}
	return nil, "", false
}

func firstUnnegatedMatch(line string, patterns []*regexp.Regexp) []int {
	for _, pattern := range patterns {
		if loc := pattern.FindStringIndex(line); loc != nil && !negatedInSentence(line, loc) {
			return loc
		}
	}
	return nil
}

// negatedInSentence reports whether the sentence holding line[loc] forbids
// the match: a prohibition cue before it ("never run --yolo"), or a
// "forbidden"/"not allowed" verdict after it.
func negatedInSentence(line string, loc []int) bool {
	start := 0
	for _, brk := range sentenceBreak.FindAllStringIndex(line[:loc[0]], -1) {
		start = brk[1]
	}
	if prohibitionCue.MatchString(line[start:loc[0]]) {
		return true
	}
	end := len(line)
	if brk := sentenceBreak.FindStringIndex(line[loc[1]:]); brk != nil {
		end = loc[1] + brk[0]
	}
	return afterProhibition.MatchString(line[loc[1]:end])
}

var afterProhibition = regexp.MustCompile(`(?i)\b(is|are)\s+(forbidden|prohibited|banned|disallowed|not (allowed|permitted))\b`)

// standingPermissionRegexFindings applies the textual standing-permission
// heuristics to a document that is not parsed structurally, reporting the
// line of each match rather than the top of the file.
func standingPermissionRegexFindings(env support.Context, file string, data []byte) []core.Finding {
	content := string(data)
	findings := make([]core.Finding, 0)
	seen := map[int]bool{}
	for _, pattern := range standingPermissionPatterns {
		for _, loc := range pattern.FindAllStringIndex(content, -1) {
			line := lineAtOffset(content, loc[0])
			if seen[line] {
				continue
			}
			seen[line] = true
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:     "prompts.agent-standing-permissions",
				Level:      "fail",
				Path:       file,
				Line:       line,
				Column:     1,
				Message:    "agent config grants standing wildcard permissions or unrestricted tool access",
				Confidence: core.ConfidenceMedium,
			}))
		}
	}
	return findings
}

// lineAtOffset returns the 1-based line of a byte offset. Patterns that may
// start with a newline anchor report the line their content begins on.
func lineAtOffset(content string, offset int) int {
	for offset < len(content) && (content[offset] == '\n' || content[offset] == '\r') {
		offset++
	}
	return strings.Count(content[:offset], "\n") + 1
}
