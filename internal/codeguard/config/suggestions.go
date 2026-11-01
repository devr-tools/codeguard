package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/suggest"
)

// outputFormats lists every report format the writers accept, in the order
// shown to users.
var outputFormats = []string{"text", "json", "sarif", "github", "github-comment", "cyclonedx", "cyclonedx-json"}

// OutputFormats returns the accepted report formats.
func OutputFormats() []string {
	return append([]string(nil), outputFormats...)
}

// ValidateOutputFormat checks a report format without loading a config, so
// commands can reject a typo such as -format sarf before any scan output. An
// empty format means "use the config's format" and is valid.
func ValidateOutputFormat(format string) error {
	normalized := strings.ToLower(strings.TrimSpace(format))
	if normalized == "" {
		return nil
	}
	for _, known := range outputFormats {
		if normalized == known {
			return nil
		}
	}
	return fmt.Errorf("unknown output format %q%s (valid formats: %s)",
		format, suggest.Hint(normalized, outputFormats), strings.Join(outputFormats, ", "))
}

// ProfileNames returns the built-in policy profile names, sorted.
func ProfileNames() []string {
	names := make([]string, 0, len(profileCatalog))
	for name := range profileCatalog {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ValidateProfileName checks a policy profile name without loading a config.
// An empty name means "no profile" and is valid.
func ValidateProfileName(profile string) error {
	normalized := normalizeProfile(profile)
	if normalized == "" {
		return nil
	}
	if _, ok := profileCatalog[normalized]; ok {
		return nil
	}
	return unknownProfileError(profile)
}

func unknownProfileError(profile string) error {
	names := ProfileNames()
	return fmt.Errorf("unknown profile %q%s (valid profiles: %s)",
		profile, suggest.Hint(normalizeProfile(profile), names), strings.Join(names, ", "))
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
