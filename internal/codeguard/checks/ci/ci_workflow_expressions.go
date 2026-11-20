package ci

import (
	"regexp"
	"strings"
)

var (
	workflowExpressionPattern = regexp.MustCompile(`(?s)\$\{\{(.*?)\}\}`)
	bracketStringAccess       = regexp.MustCompile(`\[\s*['"]([^'"\]]*)['"]\s*\]`)
	bracketIndexAccess        = regexp.MustCompile(`\[\s*(?:\d+|\*)\s*\]`)

	// Attacker-writable fields of the event payload. A fork author or any
	// commenter controls these strings, so interpolating them into a shell
	// script hands them code execution on the runner.
	untrustedEventPattern = regexp.MustCompile(`(?:^|[^a-z0-9_.-])(github\.(?:head_ref|event\.(?:` +
		`pull_request\.(?:title|body|head\.(?:ref|label)|head\.repo\.default_branch)|` +
		`comment\.body|review\.body|review_comment\.body|` +
		`head_commit\.(?:message|author\.(?:name|email))|` +
		`commits\.\*\.(?:message|author\.(?:name|email))|` +
		`discussion\.(?:title|body)|` +
		`workflow_run\.(?:head_branch|display_title|head_commit\.message)|` +
		`pages\.\*\.page_name))\b)`)
	issueFieldPattern     = regexp.MustCompile(`(?:^|[^a-z0-9_.-])(github\.event\.issue\.([a-z_*]+)(?:\.[a-z0-9_*]+)*)`)
	compositeInputPattern = regexp.MustCompile(`(?:^|[^a-z0-9_.-])(inputs\.[a-z0-9_-]+)`)
	secretsContextPattern = regexp.MustCompile(`(?:^|[^a-z0-9_.-])secrets\.([a-z0-9_]+)`)

	// Expressions that resolve to the pull request's own head commit or
	// branch rather than the base repository.
	prCodeRefPattern = regexp.MustCompile(`(?:^|[^a-z0-9_.-])(github\.(?:head_ref|event\.(?:` +
		`pull_request\.(?:head\.(?:sha|ref)|merge_commit_sha)|` +
		`workflow_run\.(?:head_sha|head_branch|head_commit\.id))))\b`)
)

// issueSafeFields are github.event.issue properties GitHub generates itself
// (numbers, timestamps, enum states, URLs); only free text such as the title,
// body and label names is attacker-controlled.
var issueSafeFields = map[string]bool{
	"number": true, "id": true, "node_id": true, "state": true, "state_reason": true,
	"locked": true, "comments": true, "created_at": true, "updated_at": true,
	"closed_at": true, "author_association": true, "html_url": true, "url": true,
	"repository_url": true, "comments_url": true, "events_url": true, "labels_url": true,
	"user": true, "assignee": true, "assignees": true, "milestone": true, "pull_request": true,
}

type workflowExpression struct {
	raw  string // the full ${{ ... }} text as written
	body string // normalized: lowercased, bracket access rewritten to dots
}

func workflowExpressions(text string) []workflowExpression {
	matches := workflowExpressionPattern.FindAllStringSubmatch(text, -1)
	out := make([]workflowExpression, 0, len(matches))
	for _, match := range matches {
		out = append(out, workflowExpression{raw: match[0], body: normalizeExpressionBody(match[1])})
	}
	return out
}

func normalizeExpressionBody(body string) string {
	normalized := strings.ToLower(strings.TrimSpace(body))
	normalized = bracketStringAccess.ReplaceAllString(normalized, ".$1")
	return bracketIndexAccess.ReplaceAllString(normalized, ".*")
}

// expressionYieldsBoolean approximates whether a single operand (one with no
// top-level && or ||) evaluates to a boolean, in which case the untrusted
// text it reads never reaches the script.
func expressionYieldsBoolean(body string) bool {
	if strings.HasPrefix(body, "!") {
		return true
	}
	for _, prefix := range []string{"contains(", "startswith(", "endswith("} {
		if strings.HasPrefix(body, prefix) {
			return true
		}
	}
	for _, operator := range []string{"==", "!=", "<=", ">="} {
		if strings.Contains(body, operator) {
			return true
		}
	}
	return false
}

// valueOperands returns the operands whose own value the expression can
// evaluate to. GitHub's && and || return one of their operands rather than a
// boolean, so `cond && github.event.comment.body || 'none'` yields the
// comment body: a comparison elsewhere in the expression must not hide it.
// Boolean operands are dropped; parentheses are looked through.
func valueOperands(body string) []string {
	body = trimEnclosingParens(strings.TrimSpace(body))
	operands := splitTopLevelLogical(body)
	if len(operands) == 1 {
		if body == "" || expressionYieldsBoolean(body) {
			return nil
		}
		return []string{body}
	}
	var out []string
	for _, operand := range operands {
		out = append(out, valueOperands(operand)...)
	}
	return out
}

// splitTopLevelLogical splits body at && and || operators that sit outside
// parentheses and single-quoted string literals.
func splitTopLevelLogical(body string) []string {
	var operands []string
	depth, start, quoted := 0, 0, false
	for i := 0; i < len(body); i++ {
		switch c := body[i]; {
		case quoted:
			if c == '\'' {
				quoted = false
			}
		case c == '\'':
			quoted = true
		case c == '(':
			depth++
		case c == ')':
			depth--
		case depth == 0 && i+1 < len(body) && (body[i:i+2] == "&&" || body[i:i+2] == "||"):
			operands = append(operands, body[start:i])
			start = i + 2
			i++
		}
	}
	return append(operands, body[start:])
}

// trimEnclosingParens removes parentheses that wrap the whole expression.
func trimEnclosingParens(body string) string {
	for strings.HasPrefix(body, "(") && strings.HasSuffix(body, ")") && closingParen(body) == len(body)-1 {
		body = strings.TrimSpace(body[1 : len(body)-1])
	}
	return body
}

// closingParen returns the index of the parenthesis matching body[0], or -1.
func closingParen(body string) int {
	depth, quoted := 0, false
	for i := 0; i < len(body); i++ {
		switch c := body[i]; {
		case quoted:
			if c == '\'' {
				quoted = false
			}
		case c == '\'':
			quoted = true
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// untrustedContext returns the first attacker-controlled context the
// expression can yield as its value, or "" when it yields none.
func untrustedContext(body string, composite bool) string {
	for _, operand := range valueOperands(body) {
		if context := untrustedOperandContext(operand, composite); context != "" {
			return context
		}
	}
	return ""
}

func untrustedOperandContext(body string, composite bool) string {
	if match := untrustedEventPattern.FindStringSubmatch(body); match != nil {
		return match[1]
	}
	for _, match := range issueFieldPattern.FindAllStringSubmatch(body, -1) {
		if !issueSafeFields[match[2]] {
			return match[1]
		}
	}
	if composite {
		if match := compositeInputPattern.FindStringSubmatch(body); match != nil {
			return match[1]
		}
	}
	return ""
}

func prCodeRef(body string) string {
	if match := prCodeRefPattern.FindStringSubmatch(body); match != nil {
		return match[1]
	}
	return ""
}

func referencedSecrets(body string) []string {
	matches := secretsContextPattern.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if match[1] == "github_token" {
			continue
		}
		out = append(out, strings.ToUpper(match[1]))
	}
	return out
}
