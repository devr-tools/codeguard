package security

import (
	"regexp"
	"strings"
)

var (
	// secretKeywordInLiteral is the loose form used on raw string literal
	// contents; precision comes from the structural conditions applied around
	// it (concatenation, key position, format directive).
	secretKeywordInLiteral = regexp.MustCompile(`(?i)password|passwd|secret|token|api[_-]?key|private[_-]?key|credential|authorization`)

	// secretFormatDirective matches "<keyword>=" / "<keyword>:" immediately
	// followed by a string-valued format directive. %d and other numeric verbs
	// are deliberately excluded so counters ("token count: %d") never fire.
	secretFormatDirective = regexp.MustCompile(`(?i)(?:password|passwd|secret|token|api[_-]?key|private[_-]?key|credential|authorization)s?["']?\s*[:=]\s*(?:%[-+ #0-9.*]*[svq]|\{)`)
)

// argLiteral is one string literal found in a raw argument list, along with
// the non-space bytes adjacent to it (0 when the literal starts or ends the
// span).
type argLiteral struct {
	content string
	before  byte
	after   byte
}

// scanArgLiterals extracts the string literals of a raw argument span.
// Backslash escapes are honored for quote and apostrophe strings; backtick
// literals (Go raw strings, JS templates) run to the closing backtick.
func scanArgLiterals(raw string) []argLiteral {
	literals := make([]argLiteral, 0, 2)
	for i := 0; i < len(raw); i++ {
		quote := raw[i]
		if quote != '"' && quote != '\'' && quote != '`' {
			continue
		}
		end := literalEnd(raw, i)
		literals = append(literals, argLiteral{
			content: raw[i+1 : end],
			before:  previousNonSpaceByte(raw, literalPrefixStart(raw, i)),
			after:   nextNonSpaceByte(raw, end+1),
		})
		i = end
	}
	return literals
}

// literalEnd returns the index of the closing quote (or the last index of raw
// when the literal is unterminated on this line).
func literalEnd(raw string, open int) int {
	quote := raw[open]
	for i := open + 1; i < len(raw); i++ {
		switch {
		case raw[i] == '\\' && quote != '`':
			i++
		case raw[i] == quote:
			return i
		}
	}
	return len(raw) - 1
}

// literalPrefixStart skips string-prefix letters (Python f/r/b) immediately
// before the opening quote so adjacency checks see the byte before the whole
// literal expression.
func literalPrefixStart(raw string, open int) int {
	start := open
	for start > 0 && (isASCIILower(raw[start-1]) || isASCIIUpper(raw[start-1])) {
		start--
	}
	if open-start > 3 {
		return open
	}
	return start
}

func previousNonSpaceByte(raw string, idx int) byte {
	for i := idx - 1; i >= 0; i-- {
		if raw[i] != ' ' && raw[i] != '\t' {
			return raw[i]
		}
	}
	return 0
}

func nextNonSpaceByte(raw string, idx int) byte {
	for i := idx; i < len(raw); i++ {
		if raw[i] != ' ' && raw[i] != '\t' {
			return raw[i]
		}
	}
	return 0
}

// literalIsSecretExposure applies the literal-based heuristics H2-H4
// documented on secretBearingArgs to one string literal.
func literalIsSecretExposure(literal argLiteral) bool {
	if secretFormatDirective.MatchString(literal.content) {
		return true // H4: "token=%s" / "password: {}"
	}
	if !secretKeywordInLiteral.MatchString(literal.content) {
		return false
	}
	if literal.before == '+' || literal.after == '+' {
		return true // H3: "Authorization: Bearer " + tok
	}
	return literal.after == ',' && literalIsSecretKey(literal.content) // H2
}

// literalIsSecretKey reports whether a literal looks like a structured-logging
// key naming a secret: a short (at most two identifier components),
// whitespace-free key such as "password", "api_key", or "auth.token".
func literalIsSecretKey(content string) bool {
	if content == "" || strings.ContainsAny(content, " \t") {
		return false
	}
	totalComponents := 0
	secret := false
	for _, ident := range a09IdentifierPattern.FindAllString(content, -1) {
		totalComponents += len(splitIdentifierComponents(ident))
		if identifierHasSecretComponent(ident) {
			secret = true
		}
	}
	return secret && totalComponents <= 2
}
