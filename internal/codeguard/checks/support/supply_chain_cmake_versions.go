package support

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	cmakeVersionPattern = regexp.MustCompile(`(?i)(?:^|[-_/])v?(\d+(?:\.\d+)+(?:[-+][A-Za-z0-9.-]+)?)(?:$|[-_/\.])`)
	cmakeExactVersion   = regexp.MustCompile(`^\d+(?:\.\d+)+(?:[-+][A-Za-z0-9.-]+)?$`)
	cmakeCommitPattern  = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)
)

func parseCMakeFindPackage(args []cmakeArgument, line int) (core.SupplyChainDependency, bool) {
	if len(args) == 0 || args[0].value == "" || containsDynamicCMakeValue(args[0].value) {
		return core.SupplyChainDependency{}, false
	}
	name := args[0].value
	version := ""
	exact := false
	if len(args) > 1 && looksLikeCMakeRequirement(args[1].value) {
		version = args[1].value
	}
	for _, arg := range args[1:] {
		if strings.EqualFold(arg.value, "EXACT") {
			exact = true
		}
	}
	return core.SupplyChainDependency{
		Name:        name,
		Requirement: version,
		Version:     version,
		Scope:       "build",
		Pinned:      exact && isExactCMakeVersion(version),
		Line:        line,
	}, true
}

func looksLikeCMakeRequirement(value string) bool {
	if value == "" || !unicode.IsDigit(rune(value[0])) {
		return false
	}
	for _, char := range value {
		if !unicode.IsDigit(char) && char != '.' && char != '<' && char != '>' && char != '=' {
			return false
		}
	}
	return true
}

func versionFromCMakeURL(url string) string {
	match := cmakeVersionPattern.FindStringSubmatch(url)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func isExactCMakeRevision(value string) bool {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	if trimmed == "" || containsDynamicCMakeValue(trimmed) {
		return false
	}
	switch lower {
	case "head", "main", "master", "develop", "development", "dev", "trunk", "latest", "stable":
		return false
	}
	return cmakeCommitPattern.MatchString(trimmed) || isExactCMakeVersion(strings.TrimPrefix(trimmed, "v"))
}

func isExactCMakeVersion(value string) bool {
	value = strings.TrimSpace(value)
	return cmakeExactVersion.MatchString(value) && !strings.ContainsAny(value, "*<>=~^,;[]")
}

func containsDynamicCMakeValue(value string) bool {
	return strings.Contains(value, "${") || strings.Contains(value, "$<") || strings.Contains(value, "$ENV{")
}
