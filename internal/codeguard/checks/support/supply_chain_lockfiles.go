package support

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type lockfileState struct {
	packages  map[string]map[string]struct{}
	selectors map[string]map[string]struct{}
}

func SupplyChainLockfileIssues(root string, manifest core.SupplyChainManifest) []string {
	if len(manifest.Lockfiles) == 0 {
		return nil
	}

	var parsedAny bool
	var firstIssues []string
	for _, lockfile := range manifest.Lockfiles {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(lockfile)))
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

func parseLockfileState(path string, data []byte) (lockfileState, bool) {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "go.sum":
		return parseGoSumState(data), true
	case "package-lock.json", "npm-shrinkwrap.json":
		return parsePackageLockState(data)
	case "pnpm-lock.yaml":
		return parsePNPMLockState(data)
	case "yarn.lock":
		return parseYarnLockState(data), true
	case "bun.lock":
		return parseBunLockState(data), true
	case "cargo.lock", "poetry.lock", "uv.lock":
		return parsePackageBlockLockState(data), true
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
		if !lockfileHasPackage(state, dep.Name) {
			issues = append(issues, "dependency "+dep.Name+" is not present in "+lockfile)
			continue
		}
		if exact := exactLockedVersion(manifest, dep); exact != "" && !lockfileHasVersion(state, dep.Name, exact) {
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
		packages:  make(map[string]map[string]struct{}),
		selectors: make(map[string]map[string]struct{}),
	}
}

func addLockfilePackage(state lockfileState, name string, version string) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if name == "" {
		return
	}
	if _, ok := state.packages[name]; !ok {
		state.packages[name] = make(map[string]struct{})
	}
	if version != "" {
		state.packages[name][version] = struct{}{}
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

func lockfileHasPackage(state lockfileState, name string) bool {
	_, ok := state.packages[name]
	return ok
}

func lockfileHasVersion(state lockfileState, name string, version string) bool {
	versions, ok := state.packages[name]
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
