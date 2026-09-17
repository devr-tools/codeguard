package support

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// lockfileState is the set of packages a lockfile pins.
//
// packages is keyed by the exact spelling the lockfile uses. normalized holds
// the same entries under their PEP 503-normalised name and is consulted only
// for ecosystems whose package names are canonicalised by the resolver (see
// ecosystemNormalizesPackageNames): npm and Cargo names are case- and
// separator-sensitive, so normalising them would silently match distinct
// packages.
type lockfileState struct {
	packages   map[string]map[string]struct{}
	selectors  map[string]map[string]struct{}
	normalized map[string]map[string]struct{}
}

// pythonNameSeparatorPattern matches the runs of separators PEP 503 collapses.
var pythonNameSeparatorPattern = regexp.MustCompile(`[-_.]+`)

func SupplyChainLockfileIssues(root string, manifest core.SupplyChainManifest) []string {
	if len(manifest.Lockfiles) == 0 {
		return nil
	}

	var parsedAny bool
	var firstIssues []string
	for _, lockfile := range manifest.Lockfiles {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(lockfile))) //nolint:gosec // lockfile path joined under the scan-target root
		if err != nil {
			continue
		}
		state, ok := parseLockfileState(lockfile, data)
		if !ok {
			continue
		}
		parsedAny = true
		issues := compareManifestToLockfile(manifest, lockfile, state)
		if len(issues) == 0 {
			return nil
		}
		if len(firstIssues) == 0 {
			firstIssues = issues
		}
	}
	if !parsedAny {
		return nil
	}
	return firstIssues
}

// parseLockfileState decodes a lockfile into the set of packages it pins.
//
// The boolean result means "this parser recognised the file", not "the file had
// packages in it". Every branch must report false when it understood nothing:
// a parser that extracted no packages from a file full of content has no basis
// for claiming that every manifest dependency is missing, and emitting a
// fail-level finding per dependency off the back of a parse failure is far
// worse than staying silent. Recognition is therefore established either by a
// structural marker of the format (an npm lockfileVersion, a [[package]] table)
// or by having extracted at least one package; a lockfile whose only content is
// blank lines and comments counts as recognised-and-empty, which is how a
// genuinely dependency-free lockfile still reports drift.
func parseLockfileState(path string, data []byte) (lockfileState, bool) {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "go.sum":
		return parseGoSumState(data)
	case "package-lock.json", "npm-shrinkwrap.json":
		return parsePackageLockState(data)
	case "pnpm-lock.yaml":
		return parsePNPMLockState(data)
	case "yarn.lock":
		return parseYarnLockState(data)
	case "bun.lock":
		return parseBunLockState(data)
	case "cargo.lock", "poetry.lock", "uv.lock":
		return parsePackageBlockLockState(data)
	case "conan.lock":
		return parseConanLockState(data)
	default:
		return lockfileState{}, false
	}
}

func compareManifestToLockfile(manifest core.SupplyChainManifest, lockfile string, state lockfileState) []string {
	issues := make([]string, 0)
	for _, dep := range manifest.Dependencies {
		if dep.Name == "" {
			continue
		}
		if !lockfileHasPackage(state, manifest.Ecosystem, dep.Name) {
			issues = append(issues, "dependency "+dep.Name+" is not present in "+lockfile)
			continue
		}
		if exact := exactLockedVersion(manifest, dep); exact != "" && !lockfileHasVersion(state, manifest.Ecosystem, dep.Name, exact) {
			if manifest.Ecosystem == "npm" && lockfileHasSelector(state, dep.Name, dep.Requirement) {
				continue
			}
			issues = append(issues, "dependency "+dep.Name+" version "+exact+" is not present in "+lockfile)
		}
	}
	return issues
}

func newLockfileState() lockfileState {
	return lockfileState{
		packages:   make(map[string]map[string]struct{}),
		selectors:  make(map[string]map[string]struct{}),
		normalized: make(map[string]map[string]struct{}),
	}
}

func addLockfilePackage(state lockfileState, name string, version string) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if name == "" {
		return
	}
	addLockfileVersion(state.packages, name, version)
	addLockfileVersion(state.normalized, normalizePythonPackageName(name), version)
}

func addLockfileVersion(index map[string]map[string]struct{}, name string, version string) {
	if name == "" {
		return
	}
	if _, ok := index[name]; !ok {
		index[name] = make(map[string]struct{})
	}
	if version != "" {
		index[name][version] = struct{}{}
	}
}

func addLockfileSelector(state lockfileState, name string, selector string) {
	name = strings.TrimSpace(name)
	selector = strings.TrimSpace(selector)
	if name == "" || selector == "" {
		return
	}
	if _, ok := state.selectors[name]; !ok {
		state.selectors[name] = make(map[string]struct{})
	}
	state.selectors[name][selector] = struct{}{}
}

func lockfileHasPackage(state lockfileState, ecosystem string, name string) bool {
	if _, ok := state.packages[name]; ok {
		return true
	}
	if !ecosystemNormalizesPackageNames(ecosystem) {
		return false
	}
	_, ok := state.normalized[normalizePythonPackageName(name)]
	return ok
}

func lockfileHasVersion(state lockfileState, ecosystem string, name string, version string) bool {
	if versions, ok := state.packages[name]; ok {
		if _, ok := versions[version]; ok {
			return true
		}
	}
	if !ecosystemNormalizesPackageNames(ecosystem) {
		return false
	}
	versions, ok := state.normalized[normalizePythonPackageName(name)]
	if !ok {
		return false
	}
	_, ok = versions[version]
	return ok
}

func lockfileHasSelector(state lockfileState, name string, selector string) bool {
	selectors, ok := state.selectors[name]
	if !ok {
		return false
	}
	_, ok = selectors[strings.TrimSpace(selector)]
	return ok
}

// ecosystemNormalizesPackageNames reports whether the ecosystem's resolver
// rewrites package names into a canonical form when it writes the lockfile, so
// that manifest and lockfile spellings must be compared through that form.
//
// Only Python does. pip, Poetry and uv all record the PEP 503-normalised name
// (`Jinja2` becomes `jinja2`, `typing_extensions` becomes `typing-extensions`)
// while manifests keep the author's spelling. npm names are case-sensitive and
// `-` and `_` name genuinely different packages, and Cargo records the crate
// name exactly as the manifest and the index spell it, so normalising either
// would match the wrong package instead of fixing a false positive.
func ecosystemNormalizesPackageNames(ecosystem string) bool {
	return ecosystem == "python"
}

// normalizePythonPackageName applies PEP 503 name normalisation: lowercase and
// collapse every run of `-`, `_` and `.` into a single `-`.
func normalizePythonPackageName(name string) string {
	return pythonNameSeparatorPattern.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
}

// lockfileHasNoContent reports whether a lockfile holds nothing but blank lines
// and comments. It is how a parser tells "this lockfile locks nothing" apart
// from "I could not read this lockfile": the former is recognised and compared,
// the latter is skipped.
func lockfileHasNoContent(data []byte) bool {
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		return false
	}
	return true
}
