package quality

import (
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// recordSlopHistory persists the artifact's score to the per-scan trend file
// under the cache directory and annotates the artifact with the previous
// score and delta when prior scans exist.
func recordSlopHistory(env support.Context, artifact *core.Artifact) {
	if artifact == nil || artifact.SlopScore == nil {
		return
	}
	// Patch validation evaluates repository-controlled content in a temporary
	// workspace and must not persist any state to repository-configured paths.
	if strings.TrimSpace(env.DiffText) != "" {
		return
	}
	cfg := env.Config.Checks.QualityRules.AIChecks
	if !aiCheckEnabled(cfg.SlopHistory) {
		return
	}
	if env.Config.Cache.Enabled != nil && !*env.Config.Cache.Enabled {
		return
	}
	path := runnersupport.SlopHistoryPathForBase(env.Config.Cache.Path)
	if path == "" {
		return
	}
	entry := core.SlopHistoryEntry{
		Timestamp:  scanTimestamp(env),
		Score:      artifact.SlopScore.Score,
		Signals:    artifact.SlopScore.Signals,
		Components: append([]core.SlopScoreComponent(nil), artifact.SlopScore.Components...),
	}
	previous, hasPrevious := runnersupport.AppendSlopHistory(path, artifact.ID, entry, cfg.SlopHistoryLimit)
	if !hasPrevious {
		return
	}
	previousScore := previous.Score
	delta := artifact.SlopScore.Score - previousScore
	artifact.SlopScore.PreviousScore = &previousScore
	artifact.SlopScore.Delta = &delta
}

func scanTimestamp(env support.Context) string {
	if !env.ScanTime.IsZero() {
		return env.ScanTime.UTC().Format(time.RFC3339)
	}
	return time.Now().UTC().Format(time.RFC3339)
}
