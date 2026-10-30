package prompts

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	// providerTokenPrefix recognizes credentials by their issuer prefix, which
	// is conclusive regardless of what the variable is called.
	providerTokenPrefix = regexp.MustCompile(`^(?:gh[pousr]_|github_pat_|glpat-|xox[abprs]-|sk-ant-|sk-|sk_live_|rk_live_|AKIA|ASIA|AIza|ya29\.|npm_|pypi-|hf_|dop_v1_|lin_api_)[0-9A-Za-z_\-.]{12,}$`)
	// secretKeyName recognizes env and header names that conventionally hold
	// a credential.
	secretKeyName = regexp.MustCompile(`(?i)(token|secret|passw(or)?d|api[_-]?key|apikey|access[_-]?key|private[_-]?key|credential|authorization|auth[_-]?key|client[_-]?secret|(^|_)pat$|bearer)`)
	// placeholderValue recognizes the documented "fill this in" shapes that
	// sample configs ship with; they are not credentials.
	placeholderValue = regexp.MustCompile(`(?i)^(x+|\*+|\.+|-+|0+)$|your[_ -]|<[^>]*>|\[[^\]]*\]|change[_-]?me|placeholder|example|redacted|replace|insert|todo|dummy|^none$|^null$|^true$|^false$`)
	authSchemePrefix = regexp.MustCompile(`(?i)^(bearer|basic|token|apikey)\s+`)
)

// literalSecretConfidence reports whether value, assigned to key in an agent
// or MCP config, is a literal credential rather than a reference to one. The
// returned confidence is high for an issuer-prefixed token and medium when
// only the key name suggests a secret.
func literalSecretConfidence(key string, value string) (string, bool) {
	value = strings.TrimSpace(authSchemePrefix.ReplaceAllString(strings.TrimSpace(value), ""))
	if value == "" || isSecretReference(value) || placeholderValue.MatchString(value) {
		return "", false
	}
	if providerTokenPrefix.MatchString(value) {
		return core.ConfidenceHigh, true
	}
	if !secretKeyName.MatchString(key) || len(value) < 12 || strings.ContainsAny(value, " \t") {
		return "", false
	}
	if strings.Trim(value, "0123456789") == "" {
		return "", false
	}
	return core.ConfidenceMedium, true
}

// isSecretReference reports whether value defers to an environment variable,
// template, or secret manager instead of carrying the secret itself.
func isSecretReference(value string) bool {
	if strings.Contains(value, "${") || strings.HasPrefix(value, "$") || strings.Contains(value, "{{") {
		return true
	}
	if strings.HasPrefix(value, "%") && strings.HasSuffix(value, "%") {
		return true
	}
	lower := strings.ToLower(value)
	for _, prefix := range []string{"op://", "env:", "vault:", "secret:", "keychain:", "aws-sm:", "gcp-sm:"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}
