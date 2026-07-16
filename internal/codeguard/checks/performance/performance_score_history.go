package performance

import (
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// recordPerformanceScoreHistory persists the artifact's score to the
// per-scan trend file next to the cache and annotates the artifact with the
// previous score and delta when prior scans exist, mirroring
// recordSlopHistory in checks/quality.
func recordPerformanceScoreHistory(env support.Context, artifact *core.Artifact) {
	if artifact == nil || artifact.PerformanceScore == nil {
		return
	}
	rules := env.Config.Checks.PerformanceRules
	if !toggleEnabled(rules.ScoreHistory) {
		return
	}
	if env.Config.Cache.Enabled != nil && !*env.Config.Cache.Enabled {
		return
	}
	path := runnersupport.PerfScoreHistoryPathForBase(env.Config.Cache.Path)
	if path == "" {
		return
	}
	entry := core.PerformanceHistoryEntry{
		Timestamp:  performanceScanTimestamp(env),
		Score:      artifact.PerformanceScore.Score,
		Signals:    artifact.PerformanceScore.Signals,
		Components: append([]core.SlopScoreComponent(nil), artifact.PerformanceScore.Components...),
	}
	previous, hasPrevious := runnersupport.AppendPerfScoreHistory(path, artifact.ID, entry, rules.ScoreHistoryLimit)
	if !hasPrevious {
		return
	}
	previousScore := previous.Score
	delta := artifact.PerformanceScore.Score - previousScore
	artifact.PerformanceScore.PreviousScore = &previousScore
	artifact.PerformanceScore.Delta = &delta
}

func performanceScanTimestamp(env support.Context) string {
	if !env.ScanTime.IsZero() {
		return env.ScanTime.UTC().Format(time.RFC3339)
	}
	return time.Now().UTC().Format(time.RFC3339)
}
