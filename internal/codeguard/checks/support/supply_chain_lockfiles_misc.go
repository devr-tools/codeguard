package support

import (
	"regexp"
	"sort"
	"strings"
)

var (
	bunCoordinatePattern = regexp.MustCompile(`(?:@[-a-zA-Z0-9_.]+/)?[-a-zA-Z0-9_.]+@[0-9][0-9A-Za-z.+_-]*`)
	// packageBlockLockHeaderPattern matches the only lines Cargo, Poetry and uv
	// write into a lockfile that pins nothing: the format version, uv's
	// revision/requires-python, Poetry's [metadata] block, and the explicit
	// empty `package = []`.
	packageBlockLockHeaderPattern = regexp.MustCompile(`^(?:\[metadata\]|version\s*=\s*\d+|revision\s*=\s*\d+|lock-version\s*=|content-hash\s*=|requires-python\s*=|packages?\s*=\s*\[\s*\])`)
)

// parseGoSumState reads go.sum. The format carries no header, so recognition
// rests on having read at least one `module version hash` line (or on the file
// being empty, which is a real go.sum for a dependency-free module).
func parseGoSumState(data []byte) (lockfileState, bool) {
	state := newLockfileState()
	for _, rawLine := range strings.Split(string(data), "\n") {
		fields := strings.Fields(strings.TrimSpace(rawLine))
		if len(fields) >= 2 {
			addLockfilePackage(state, fields[0], strings.TrimSuffix(fields[1], "/go.mod"))
		}
	}
	return state, len(state.packages) > 0 || lockfileHasNoContent(data)
}

// parsePackageBlockLockState reads the TOML `[[package]]` table layout shared
// by Cargo.lock, poetry.lock and uv.lock.
//
// A `[[package]]` table is the structural marker that the file is one of these.
// A file with none is treated as empty — rather than unparsed — only when every
// substantive line in it is a known header key, which a truncated or
// conflict-marked lockfile would fail, so a real-but-unreadable lockfile cannot
// slip through as "empty" and be reported as total drift.
func parsePackageBlockLockState(data []byte) (lockfileState, bool) {
	state := newLockfileState()
	currentName := ""
	currentVersion := ""
	inPackage := false
	sawPackageTable := false
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
			sawPackageTable = true
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
	return state, sawPackageTable || packageBlockLockHeaderOnly(data)
}

// packageBlockLockHeaderOnly reports whether the file contains nothing but
// comments and the header keys these lockfile formats emit when they pin no
// packages, which makes it a genuinely empty lockfile rather than one this
// parser failed to read.
func packageBlockLockHeaderOnly(data []byte) bool {
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !packageBlockLockHeaderPattern.MatchString(line) {
			return false
		}
	}
	return true
}

// parseBunLockState scrapes `name@version` coordinates out of bun.lock (JSONC,
// which is not valid JSON, hence the scrape). Having found a coordinate is the
// only evidence that the scrape worked.
func parseBunLockState(data []byte) (lockfileState, bool) {
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
	return state, len(state.packages) > 0 || lockfileHasNoContent(data)
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
