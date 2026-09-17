package support

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// yarnRegistryProtocol is the only Yarn Berry resolution protocol whose
// requirement is a public registry range. Anything else (workspace:, link:,
// portal:, file:, patch:, git:, exec:, …) resolves to a local path or a
// rewritten package, so its "requirement" is not a version range and must not
// be recorded as one.
const yarnRegistryProtocol = "npm"

func parsePackageLockState(data []byte) (lockfileState, bool) {
	var doc struct {
		LockfileVersion *int `json:"lockfileVersion"`
		Packages        map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
		Dependencies map[string]json.RawMessage `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return lockfileState{}, false
	}
	// An npm lockfile identifies itself structurally, so a freshly generated
	// one with no dependencies is still recognised (and therefore compared).
	recognized := doc.LockfileVersion != nil || doc.Packages != nil || doc.Dependencies != nil
	state := newLockfileState()
	for pkgPath, pkg := range doc.Packages {
		name := packageLockEntryName(pkgPath)
		if name != "" {
			addLockfilePackage(state, name, strings.TrimSpace(pkg.Version))
		}
	}
	for name, raw := range doc.Dependencies {
		var dep struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(raw, &dep); err == nil {
			addLockfilePackage(state, name, strings.TrimSpace(dep.Version))
		}
	}
	return state, recognized || len(state.packages) > 0
}

func parsePNPMLockState(data []byte) (lockfileState, bool) {
	var doc struct {
		LockfileVersion any            `yaml:"lockfileVersion"`
		Importers       map[string]any `yaml:"importers"`
		Packages        map[string]any `yaml:"packages"`
		Snapshots       map[string]any `yaml:"snapshots"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return lockfileState{}, false
	}
	recognized := doc.LockfileVersion != nil || doc.Importers != nil || doc.Packages != nil || doc.Snapshots != nil
	state := newLockfileState()
	for _, keys := range []map[string]any{doc.Packages, doc.Snapshots} {
		for key := range keys {
			name, version := parsePNPMPackageKey(key)
			if name != "" {
				addLockfilePackage(state, name, version)
			}
		}
	}
	return state, recognized || len(state.packages) > 0
}

// parseYarnLockState reads both Yarn v1 ("classic") and Yarn 2+ ("Berry")
// lockfiles. The two share a block layout — an unindented selector line ending
// in `:` followed by indented fields — but spell the version field
// differently:
//
//	classic:  version "2.6.0"
//	berry:    version: 2.6.0
//
// Reading only the classic spelling parsed every Berry lockfile to zero
// packages, which surfaced as a fail-level drift finding for every dependency
// in the manifest.
//
// Recognition deliberately does not trust Berry's `__metadata:` header: yarn
// never writes a lockfile full of entries that yield no packages, so "zero
// packages from a non-empty file" means this parser failed, and treating the
// header as proof of an empty lockfile would let exactly that failure be
// reported as total drift again.
func parseYarnLockState(data []byte) (lockfileState, bool) {
	state := newLockfileState()
	currentSelectors := make([]string, 0)
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			currentSelectors = nil
		case !strings.HasPrefix(rawLine, " ") && !strings.HasPrefix(rawLine, "\t") && strings.HasSuffix(line, ":"):
			currentSelectors = parseYarnSelectors(strings.TrimSuffix(line, ":"))
		case len(currentSelectors) > 0:
			version, ok := parseYarnVersionLine(line)
			if !ok {
				continue
			}
			for _, selector := range currentSelectors {
				name, req := parseYarnSelector(selector)
				if name == "" {
					continue
				}
				addLockfilePackage(state, name, version)
				addLockfileSelector(state, name, req)
			}
		}
	}
	return state, len(state.packages) > 0 || lockfileHasNoContent(data)
}

// parseYarnVersionLine extracts the resolved version from a lockfile entry's
// version field in either the classic or the Berry spelling.
func parseYarnVersionLine(line string) (string, bool) {
	switch {
	case strings.HasPrefix(line, "version:"):
		value := strings.TrimSpace(strings.TrimPrefix(line, "version:"))
		return strings.Trim(value, `"'`), true
	case strings.HasPrefix(line, "version "):
		return firstQuotedValue(line), true
	default:
		return "", false
	}
}

func packageLockEntryName(entry string) string {
	entry = filepath.ToSlash(strings.TrimSpace(entry))
	if entry == "" || entry == "node_modules" {
		return ""
	}
	if idx := strings.LastIndex(entry, "node_modules/"); idx >= 0 {
		return strings.TrimPrefix(entry[idx:], "node_modules/")
	}
	return ""
}

// parsePNPMPackageKey splits a pnpm `packages`/`snapshots` key into name and
// version. pnpm has used two shapes: `/name/1.2.3` (lockfile v5 and earlier,
// optionally with a `_peer-hash` suffix) and `name@1.2.3` (v6 and later,
// optionally with `(peer@1.0.0)` suffixes). Only the second was handled, so v5
// lockfiles also parsed to zero packages.
func parsePNPMPackageKey(key string) (string, string) {
	key = strings.TrimSpace(strings.TrimPrefix(key, "/"))
	if key == "" {
		return "", ""
	}
	if idx := strings.Index(key, "("); idx >= 0 {
		key = key[:idx]
	}
	if slash := strings.LastIndex(key, "/"); slash > 0 && startsWithDigit(key[slash+1:]) {
		version := key[slash+1:]
		if under := strings.Index(version, "_"); under > 0 {
			version = version[:under]
		}
		return key[:slash], version
	}
	at := strings.LastIndex(key, "@")
	if at <= 0 {
		return "", ""
	}
	return key[:at], key[at+1:]
}

func parseYarnSelectors(line string) []string {
	parts := strings.Split(line, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, `"`))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// parseYarnSelector splits a lockfile selector into the package name and the
// registry range it was requested with.
//
// The name ends at the first `@` that is not the leading one of a scope, which
// is correct for classic (`left-pad@^1.3.0`, `@babel/core@^7.0.0`) and for
// Berry's protocol form (`@linear/sdk@npm:^2.6.0`,
// `fsevents@patch:fsevents@npm%3A2.3.2#…`) alike; splitting on the last `@`
// mis-read both Berry protocols and classic npm aliases.
//
// The returned requirement is empty for any non-registry protocol: a
// workspace:, link:, portal:, file:, patch: or git: resolution is a local or
// rewritten package, not a registry range, and comparing a manifest range
// against it would be meaningless. The package is still recorded as present
// with its resolved version, so such a dependency is never reported as missing.
func parseYarnSelector(selector string) (string, string) {
	selector = strings.TrimSpace(strings.Trim(selector, `"`))
	at := yarnSelectorNameEnd(selector)
	if at <= 0 {
		return "", ""
	}
	name := selector[:at]
	requirement := selector[at+1:]
	protocol, value, ok := splitYarnProtocol(requirement)
	if !ok {
		return name, requirement
	}
	if protocol != yarnRegistryProtocol {
		return name, ""
	}
	// `npm:<other-name>@<range>` is an alias; the range is the trailing part.
	if alias := yarnSelectorNameEnd(value); alias > 0 {
		value = value[alias+1:]
	}
	return name, value
}

// yarnSelectorNameEnd returns the index of the `@` separating a selector's
// package name from its requirement, or -1 when there is none. A leading `@`
// belongs to the package scope and is skipped.
func yarnSelectorNameEnd(selector string) int {
	if len(selector) < 2 {
		return -1
	}
	idx := strings.Index(selector[1:], "@")
	if idx < 0 {
		return -1
	}
	return idx + 1
}

// splitYarnProtocol separates a Yarn Berry protocol prefix ("npm:^2.6.0" ->
// "npm", "^2.6.0") from the value it qualifies. A classic semver range has no
// protocol; a URL-shaped requirement reports its scheme as the protocol, which
// is correct — it is not a registry range either.
func splitYarnProtocol(requirement string) (string, string, bool) {
	idx := strings.Index(requirement, ":")
	if idx <= 0 {
		return "", requirement, false
	}
	protocol := requirement[:idx]
	if len(protocol) > 12 {
		return "", requirement, false
	}
	for _, r := range protocol {
		if (r < 'a' || r > 'z') && r != '+' && r != '-' && r != '.' {
			return "", requirement, false
		}
	}
	return protocol, requirement[idx+1:], true
}
