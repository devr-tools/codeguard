package security

import (
	"bytes"
	"math"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type entropySettings struct {
	enabled   bool
	minLength int
	threshold float64
	level     string
}

const (
	defaultEntropyMinLength = 20
	defaultEntropyThreshold = 4.5
)

func buildEntropySettings(cfg *core.SecretsEntropyConfig) entropySettings {
	settings := entropySettings{minLength: defaultEntropyMinLength, threshold: defaultEntropyThreshold, level: "warn"}
	if cfg == nil {
		return settings
	}
	if cfg.Enabled != nil {
		settings.enabled = *cfg.Enabled
	}
	if cfg.MinLength > 0 {
		settings.minLength = cfg.MinLength
	}
	if cfg.Threshold > 0 {
		settings.threshold = cfg.Threshold
	}
	settings.level = normalizeSecretLevel(cfg.Level, "warn")
	return settings
}

func normalizeSecretLevel(level string, fallback string) string {
	switch strings.TrimSpace(strings.ToLower(level)) {
	case "warn":
		return "warn"
	case "fail":
		return "fail"
	default:
		return fallback
	}
}

// entropyMatch reports the first high-entropy quoted literal on the line, if the
// (opt-in) entropy heuristic is configured to fire on it.
func (s Scanner) entropyMatch(lineNo int, line string) *Match {
	for _, found := range quotedLiteralPattern.FindAllStringSubmatch(line, -1) {
		value := found[1]
		if len([]rune(value)) < s.entropy.minLength {
			continue
		}
		if isPlaceholderSecret(value) {
			continue
		}
		if shannonEntropy(value) < s.entropy.threshold {
			continue
		}
		return &Match{RuleID: highEntropyRule, Level: s.entropy.level, Line: lineNo, Column: 1, Message: "high-entropy string literal (possible secret): " + maskSecret(value)}
	}
	return nil
}

// credentialMatchValue returns the captured secret value when the pattern has a
// capture group, otherwise the whole match (a self-contained token).
func credentialMatchValue(match []string) string {
	if len(match) > 1 && strings.TrimSpace(match[len(match)-1]) != "" {
		return match[len(match)-1]
	}
	return match[0]
}

func isPlaceholderSecret(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}
	if placeholderPattern.MatchString(trimmed) {
		return true
	}
	lower := strings.ToLower(trimmed)
	switch {
	case strings.Contains(lower, "example"),
		strings.Contains(lower, "redacted"),
		strings.Contains(lower, "placeholder"),
		strings.Contains(lower, "your_"),
		strings.Contains(lower, "your-"),
		strings.Contains(lower, "op://"),    // 1Password secret reference, not a literal
		strings.Contains(lower, "vault://"), // Vault secret reference
		strings.HasPrefix(trimmed, "$("):    // shell command substitution
		return true
	}
	return allSameRune(trimmed)
}

func allSameRune(value string) bool {
	if len(value) < 2 {
		return false
	}
	first := rune(value[0])
	for _, r := range value {
		if r != first {
			return false
		}
	}
	return true
}

// maskSecret renders a secret value for display without reprinting it in full,
// keeping enough context (first/last four characters) to locate it.
func maskSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	runes := []rune(trimmed)
	if len(runes) <= 8 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:4]) + "…" + string(runes[len(runes)-4:])
}

// looksBinary reports whether data appears to be binary (contains a NUL byte in
// its leading window). Binary files are skipped: scanning them wastes time and
// produces noise rather than real credential findings.
func looksBinary(data []byte) bool {
	limit := len(data)
	if limit > binarySniffBytes {
		limit = binarySniffBytes
	}
	return bytes.IndexByte(data[:limit], 0) >= 0
}

// shannonEntropy returns the Shannon entropy of s in bits per character. ASCII
// bytes are counted in a fixed array to avoid a per-call map allocation; the
// rare non-ASCII rune falls back to a map.
func shannonEntropy(s string) float64 {
	if s == "" {
		return 0
	}
	var ascii [256]int
	var wide map[rune]int
	total := 0
	for _, r := range s {
		total++
		if r < 256 {
			ascii[r]++
			continue
		}
		if wide == nil {
			wide = make(map[rune]int)
		}
		wide[r]++
	}
	if total == 0 {
		return 0
	}
	ftotal := float64(total)
	entropy := 0.0
	for _, count := range ascii {
		if count == 0 {
			continue
		}
		p := float64(count) / ftotal
		entropy -= p * math.Log2(p)
	}
	for _, count := range wide {
		p := float64(count) / ftotal
		entropy -= p * math.Log2(p)
	}
	return entropy
}
