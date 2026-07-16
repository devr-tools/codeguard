package performance

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// maxStatsFileBytes caps how much of a bundler stats JSON is read into
// memory: the stats file lives in the scanned repository, which may be an
// untrusted pull request, so the read is bounded like every other repository
// input.
const maxStatsFileBytes = 16 << 20 // 16 MiB

func bundleStatsBudgetFindings(env support.Context, target core.TargetConfig, budget core.PerformanceBudgetConfig) []core.Finding {
	paths, finding := resolveBudgetArtifacts(env, target, budget)
	if finding != nil {
		return []core.Finding{*finding}
	}
	stats, err := readBundleStats(paths[0])
	if err != nil {
		return []core.Finding{budgetIssueFinding(env, budget, fmt.Sprintf("stats file %q: %v; budget skipped", budget.Path, err))}
	}
	if budget.Asset != "" {
		size, ok := stats.assets[budget.Asset]
		if !ok {
			return []core.Finding{budgetIssueFinding(env, budget, fmt.Sprintf("asset %q not found in stats file %q; budget skipped", budget.Asset, budget.Path))}
		}
		if size <= budget.MaxBytes {
			return nil
		}
		return []core.Finding{budgetExceededFinding(env, budget, fmt.Sprintf("asset %q is %d bytes", budget.Asset, size))}
	}
	if stats.total <= budget.MaxBytes {
		return nil
	}
	return []core.Finding{budgetExceededFinding(env, budget, fmt.Sprintf("assets in %q total %d bytes", budget.Path, stats.total))}
}

// bundleStats is the minimal shape shared by the supported stats formats: a
// per-asset size map and the total across assets.
type bundleStats struct {
	total  int64
	assets map[string]int64
}

// readBundleStats reads and parses a bundler stats JSON, supporting the two
// common minimal shapes: an esbuild metafile (outputs.<name>.bytes) and a
// webpack stats file (assets[].size).
func readBundleStats(path string) (bundleStats, error) {
	info, err := os.Stat(path) //nolint:gosec // containment verified by caller
	if err != nil {
		return bundleStats{}, err
	}
	if info.Size() > maxStatsFileBytes {
		return bundleStats{}, fmt.Errorf("stats file is %d bytes, larger than the %d byte limit", info.Size(), maxStatsFileBytes)
	}
	f, err := os.Open(path) //nolint:gosec // containment verified by caller
	if err != nil {
		return bundleStats{}, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxStatsFileBytes))
	if err != nil {
		return bundleStats{}, err
	}
	return parseBundleStats(data)
}

func parseBundleStats(data []byte) (bundleStats, error) {
	if stats, ok := parseEsbuildMetafile(data); ok {
		return stats, nil
	}
	if stats, ok := parseWebpackStats(data); ok {
		return stats, nil
	}
	return bundleStats{}, fmt.Errorf("unrecognized stats format (expected an esbuild metafile with outputs.<name>.bytes or webpack stats with assets[].size)")
}

func parseEsbuildMetafile(data []byte) (bundleStats, bool) {
	var metafile struct {
		Outputs map[string]struct {
			Bytes int64 `json:"bytes"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal(data, &metafile); err != nil || len(metafile.Outputs) == 0 {
		return bundleStats{}, false
	}
	stats := bundleStats{assets: make(map[string]int64, len(metafile.Outputs))}
	for name, output := range metafile.Outputs {
		stats.assets[name] = output.Bytes
		stats.total += output.Bytes
	}
	return stats, true
}

func parseWebpackStats(data []byte) (bundleStats, bool) {
	var webpack struct {
		Assets []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(data, &webpack); err != nil || len(webpack.Assets) == 0 {
		return bundleStats{}, false
	}
	stats := bundleStats{assets: make(map[string]int64, len(webpack.Assets))}
	for _, asset := range webpack.Assets {
		stats.assets[asset.Name] = asset.Size
		stats.total += asset.Size
	}
	return stats, true
}
