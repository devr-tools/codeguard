package security

import (
	"regexp"
	"strings"
)

var a09IdentifierPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// secretIdentifierComponents are the normalized identifier components that
// mark a value as secret-bearing. "apikey"/"privatekey" also match as the
// adjacent component pairs api+key / private+key.
var secretIdentifierComponents = map[string]bool{
	"password":      true,
	"passwd":        true,
	"secret":        true,
	"token":         true,
	"apikey":        true,
	"privatekey":    true,
	"credential":    true,
	"authorization": true,
}

// textHasSecretIdentifier reports whether any identifier in text has a
// secret-named component.
func textHasSecretIdentifier(text string) bool {
	for _, ident := range a09IdentifierPattern.FindAllString(text, -1) {
		if identifierHasSecretComponent(ident) {
			return true
		}
	}
	return false
}

// identifierHasSecretComponent splits an identifier into snake_case and
// camelCase components and reports whether any component — with trailing
// digits and a plural "s" stripped — is a secret keyword, or whether adjacent
// components form "api key" / "private key". Matching whole components keeps
// words that merely embed a keyword (e.g. "tokenizer") from firing.
func identifierHasSecretComponent(ident string) bool {
	components := splitIdentifierComponents(ident)
	previous := ""
	for _, component := range components {
		normalized := normalizeSecretComponent(component)
		if secretIdentifierComponents[normalized] {
			return true
		}
		if normalized == "key" && (previous == "api" || previous == "private") {
			return true
		}
		previous = normalized
	}
	return false
}

func normalizeSecretComponent(component string) string {
	component = strings.ToLower(component)
	component = strings.TrimRight(component, "0123456789")
	if len(component) > 1 {
		component = strings.TrimSuffix(component, "s")
	}
	return component
}

// splitIdentifierComponents splits an ASCII identifier at underscores and
// camelCase boundaries: "userPassword" -> [user, Password], "api_key" ->
// [api, key], "APIKey" -> [API, Key].
func splitIdentifierComponents(ident string) []string {
	components := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(ident); i++ {
		if ident[i] == '_' {
			if i > start {
				components = append(components, ident[start:i])
			}
			start = i + 1
			continue
		}
		if i > start && isCamelBoundary(ident, i) {
			components = append(components, ident[start:i])
			start = i
		}
	}
	if start < len(ident) {
		components = append(components, ident[start:])
	}
	return components
}

// isCamelBoundary reports a camelCase component boundary before index i: a
// lower/digit-to-upper transition, or the final upper of an acronym run
// followed by a lowercase letter ("APIKey" splits before "Key").
func isCamelBoundary(ident string, i int) bool {
	if !isASCIIUpper(ident[i]) {
		return false
	}
	prev := ident[i-1]
	if isASCIILower(prev) || isASCIIDigit(prev) {
		return true
	}
	return isASCIIUpper(prev) && i+1 < len(ident) && isASCIILower(ident[i+1])
}

func isASCIIUpper(b byte) bool { return b >= 'A' && b <= 'Z' }
func isASCIILower(b byte) bool { return b >= 'a' && b <= 'z' }
func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }
