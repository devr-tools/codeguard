package agentcontext

import (
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// recordLegibilityHistory persists the artifact's score to the per-scan trend
// file next to the cache (<cache>.legibility-history.<ext>) and annotates the
// artifact with the previous score and delta when prior scans exist,
// mirroring recordSlopHistory in checks/quality and
// recordPerformanceScoreHistory in checks/performance.
func recordLegibilityHistory(env support.Context, artifact *core.Artifact) {
	if artifact == nil || artifact.RepoLegibility == nil {
		return
	}
	rules := env.Config.Checks.ContextRules
	if rules.LegibilityHistory != nil && !*rules.LegibilityHistory {
		return
	}
	if env.Config.Cache.Enabled != nil && !*env.Config.Cache.Enabled {
		return
	}
	path := runnersupport.LegibilityHistoryPathForBase(env.Config.Cache.Path)
	if path == "" {
		return
	}
	entry := core.LegibilityHistoryEntry{
		Timestamp:  legibilityScanTimestamp(env),
		Score:      artifact.RepoLegibility.Score,
		Components: append([]core.RepoLegibilityComponent(nil), artifact.RepoLegibility.Components...),
	}
	previous, hasPrevious := runnersupport.AppendLegibilityHistory(path, artifact.ID, entry, rules.LegibilityHistoryLimit)
	if !hasPrevious {
		return
	}
	previousScore := previous.Score
	delta := artifact.RepoLegibility.Score - previousScore
	artifact.RepoLegibility.PreviousScore = &previousScore
	artifact.RepoLegibility.Delta = &delta
}

func legibilityScanTimestamp(env support.Context) string {
	if !env.ScanTime.IsZero() {
		return env.ScanTime.UTC().Format(time.RFC3339)
	}
	return time.Now().UTC().Format(time.RFC3339)
}
