package quality

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// listAITargetFiles returns the target files matching include, sharing the
// per-scan corpus walk when the runner wired the hook and falling back to a
// direct walk for unit-test contexts. Both paths apply the same configured
// excludes and return nil when the walk fails, matching the historical
// WalkFiles-based behavior.
func listAITargetFiles(env support.Context, target core.TargetConfig, include func(string) bool) []string {
	if env.ListTargetFiles != nil {
		all, err := env.ListTargetFiles(target)
		if err != nil {
			return nil
		}
		files := make([]string, 0, len(all))
		for _, rel := range all {
			if include(rel) {
				files = append(files, rel)
			}
		}
		return files
	}
	files, err := runnersupport.WalkFiles(target.Path, env.Config.Exclude, include)
	if err != nil {
		return nil
	}
	return files
}

// readAITargetFile reads a walk-enumerated file under the target root through
// the shared per-scan corpus when the runner wired the hook, so the AI checks
// no longer re-read the full source corpus that other sections already loaded.
// Files reaching this path came from the corpus walk, which already enforces
// the scan size cap, so the capped corpus read behaves identically to the
// direct os.ReadFile it replaces.
func readAITargetFile(env support.Context, target core.TargetConfig, rel string) ([]byte, error) {
	if env.ReadTargetFile != nil {
		return env.ReadTargetFile(target, rel)
	}
	return os.ReadFile(filepath.Join(target.Path, rel)) //nolint:gosec // path resolved under the scan-target root
}

// aiTargetSourceFiles lists the target files whose lowercased path ends with
// one of the given suffixes, honoring configured excludes.
func aiTargetSourceFiles(env support.Context, target core.TargetConfig, suffixes ...string) []string {
	return listAITargetFiles(env, target, func(rel string) bool {
		lower := strings.ToLower(rel)
		for _, suffix := range suffixes {
			if strings.HasSuffix(lower, suffix) {
				return true
			}
		}
		return false
	})
}
