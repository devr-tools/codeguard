package support

import (
	"regexp"
	"sort"
	"strings"
)

var bunCoordinatePattern = regexp.MustCompile(`(?:@[-a-zA-Z0-9_.]+/)?[-a-zA-Z0-9_.]+@[0-9][0-9A-Za-z.+_-]*`)

func parseGoSumState(data []byte) lockfileState {
	state := newLockfileState()
	for _, rawLine := range strings.Split(string(data), "\n") {
		fields := strings.Fields(strings.TrimSpace(rawLine))
		if len(fields) >= 2 {
			addLockfilePackage(state, fields[0], strings.TrimSuffix(fields[1], "/go.mod"))
		}
	}
	return state
}

func parsePackageBlockLockState(data []byte) lockfileState {
	state := newLockfileState()
	currentName := ""
	currentVersion := ""
	inPackage := false
	flush := func() {
		if currentName != "" {
			addLockfilePackage(state, currentName, currentVersion)
		}
		currentName = ""
		currentVersion = ""
	}
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case line == "[[package]]":
			flush()
			inPackage = true
		case inPackage && strings.HasPrefix(line, "name") && strings.Contains(line, "="):
			currentName = parseTOMLAssignmentValue(line)
		case inPackage && strings.HasPrefix(line, "version") && strings.Contains(line, "="):
			currentVersion = parseTOMLAssignmentValue(line)
		case inPackage && strings.HasPrefix(line, "[["):
			flush()
			inPackage = false
		}
	}
	flush()
	return state
}

func parseBunLockState(data []byte) lockfileState {
	state := newLockfileState()
	seen := map[string]struct{}{}
	quoted := quotedStringPattern.FindAllStringSubmatch(string(data), -1)
	for _, match := range quoted {
		addBunCoordinate(state, seen, match[1])
	}
	tokens := bunCoordinatePattern.FindAllString(string(data), -1)
	sort.Strings(tokens)
	for _, token := range tokens {
		addBunCoordinate(state, seen, token)
	}
	return state
}

func addBunCoordinate(state lockfileState, seen map[string]struct{}, token string) {
	name, version := parsePackageCoordinate(token)
	if name == "" || version == "" {
		return
	}
	key := name + "@" + version
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	addLockfilePackage(state, name, version)
}

func parsePackageCoordinate(token string) (string, string) {
	token = strings.TrimSpace(strings.Trim(token, "\"'`,:;()[]{}"))
	if token == "" {
		return "", ""
	}
	at := strings.LastIndex(token, "@")
	if at <= 0 || at >= len(token)-1 {
		return "", ""
	}
	name := strings.TrimSpace(token[:at])
	version := strings.TrimSpace(token[at+1:])
	if name == "" || version == "" || !startsWithDigit(version) {
		return "", ""
	}
	return name, version
}

func startsWithDigit(value string) bool {
	return value != "" && value[0] >= '0' && value[0] <= '9'
}

func parseTOMLAssignmentValue(line string) string {
	if idx := strings.Index(line, "="); idx >= 0 {
		return strings.TrimSpace(strings.Trim(parseTOMLQuotedValue(line[idx+1:]), `"`))
	}
	return ""
}

func parseTOMLQuotedValue(value string) string {
	if match := regexp.MustCompile(`["']([^"']+)["']`).FindStringSubmatch(value); match != nil {
		return strings.TrimSpace(match[1])
	}
	return strings.TrimSpace(value)
}
