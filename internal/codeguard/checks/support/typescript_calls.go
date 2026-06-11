package support

import (
	"regexp"
	"strings"
)

type ScriptCall struct {
	Line   int
	Offset int
	Args   []string
}

func FindScriptCalls(source string, code string, pattern *regexp.Regexp) []ScriptCall {
	matches := pattern.FindAllStringIndex(code, -1)
	if len(matches) == 0 {
		return nil
	}

	calls := make([]ScriptCall, 0, len(matches))
	seen := make(map[int]struct{}, len(matches))
	for _, match := range matches {
		openParen := callOpenParen(code, match[0], match[1])
		if openParen < 0 {
			continue
		}
		if _, exists := seen[openParen]; exists {
			continue
		}
		seen[openParen] = struct{}{}
		args := parseScriptCallArguments(source, openParen)
		calls = append(calls, ScriptCall{
			Line:   LineNumberForOffset(source, openParen),
			Offset: openParen,
			Args:   args,
		})
	}
	return calls
}

func callOpenParen(code string, start int, end int) int {
	for idx := start; idx < end && idx < len(code); idx++ {
		if code[idx] == '(' {
			return idx
		}
	}
	return -1
}

func HasObjectLiteralBooleanFlag(argument string, key string, expected bool) bool {
	value := "false"
	if expected {
		value = "true"
	}
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(strings.TrimSpace(key)) + `\s*:\s*` + value + `\b`)
	return pattern.MatchString(argument)
}

func HasStringLiteralValue(argument string, values ...string) bool {
	unquoted, ok := UnquoteSimpleScriptString(argument)
	if !ok {
		return false
	}
	if len(values) == 0 {
		return true
	}
	for _, value := range values {
		if unquoted == value {
			return true
		}
	}
	return false
}

func UnquoteSimpleScriptString(argument string) (string, bool) {
	argument = strings.TrimSpace(argument)
	if len(argument) < 2 {
		return "", false
	}
	if argument[0] != argument[len(argument)-1] {
		return "", false
	}
	switch argument[0] {
	case '\'', '"', '`':
		content := argument[1 : len(argument)-1]
		if strings.Contains(content, "${") {
			return "", false
		}
		return content, true
	default:
		return "", false
	}
}
