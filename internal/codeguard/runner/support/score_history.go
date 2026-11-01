package support

import (
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// SectionDiagnostic is an operational diagnostic raised after the sections
// have finalized, paired with the ID of the section it belongs to.
type SectionDiagnostic struct {
	SectionID  string
	Diagnostic core.Diagnostic
}

// RecordScoreHistories persists the trend history of every score artifact
// the sections published (slop_score, performance_score, repo_legibility) and
// annotates each artifact in sc.Artifacts with the previous score and delta
// when an earlier scan was recorded. It runs once, after every section has
// finalized, so the checks that compute the scores stay free of side effects.
// A history file that cannot be written yields a scan.history-write
// diagnostic for the section that owns the artifact.
func RecordScoreHistories(sc Context) []SectionDiagnostic {
	if !scoreHistoryCacheEnabled(sc.Cfg) {
		return nil
	}
	var diagnostics []SectionDiagnostic
	for _, artifact := range sc.Artifacts.List() {
		var diagnostic *SectionDiagnostic
		switch {
		case artifact.SlopScore != nil:
			diagnostic = recordSlopScoreHistory(sc, &artifact)
		case artifact.PerformanceScore != nil:
			diagnostic = recordPerformanceScoreHistory(sc, &artifact)
		case artifact.RepoLegibility != nil:
			diagnostic = recordLegibilityScoreHistory(sc, &artifact)
		default:
			continue
		}
		sc.Artifacts.Put(artifact)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
		}
	}
	return diagnostics
}

// scoreHistoryCacheEnabled mirrors the score-history gate: only an explicit
// cache.enabled=false disables persistence.
func scoreHistoryCacheEnabled(cfg core.Config) bool {
	return cfg.Cache.Enabled == nil || *cfg.Cache.Enabled
}

func historyToggleEnabled(flag *bool) bool {
	return flag == nil || *flag
}

func scoreHistoryTimestamp(sc Context) string {
	if !sc.Today.IsZero() {
		return sc.Today.UTC().Format(time.RFC3339)
	}
	return time.Now().UTC().Format(time.RFC3339)
}

func historyWriteSectionDiagnostic(sectionID string, history string, path string, err error) *SectionDiagnostic {
	if err == nil {
		return nil
	}
	return &SectionDiagnostic{SectionID: sectionID, Diagnostic: HistoryWriteDiagnostic(history, path, err)}
}

func recordSlopScoreHistory(sc Context, artifact *core.Artifact) *SectionDiagnostic {
	// Patch validation evaluates repository-controlled content in a temporary
	// workspace and must not persist any state to repository-configured paths.
	if strings.TrimSpace(sc.Opts.DiffText) != "" {
		return nil
	}
	cfg := sc.Cfg.Checks.QualityRules.AIChecks
	if !historyToggleEnabled(cfg.SlopHistory) {
		return nil
	}
	path := SlopHistoryPathForBase(sc.Cfg.Cache.Path)
	if path == "" {
		return nil
	}
	score := *artifact.SlopScore
	entry := core.SlopHistoryEntry{
		Timestamp:  scoreHistoryTimestamp(sc),
		Score:      score.Score,
		Signals:    score.Signals,
		Components: append([]core.SlopScoreComponent(nil), score.Components...),
	}
	previous, hasPrevious, err := AppendSlopHistory(path, artifact.ID, entry, cfg.SlopHistoryLimit)
	if hasPrevious {
		previousScore := previous.Score
		delta := score.Score - previousScore
		score.PreviousScore = &previousScore
		score.Delta = &delta
	}
	artifact.SlopScore = &score
	return historyWriteSectionDiagnostic("quality", "slop", path, err)
}

func recordPerformanceScoreHistory(sc Context, artifact *core.Artifact) *SectionDiagnostic {
	rules := sc.Cfg.Checks.PerformanceRules
	if !historyToggleEnabled(rules.ScoreHistory) {
		return nil
	}
	path := PerfScoreHistoryPathForBase(sc.Cfg.Cache.Path)
	if path == "" {
		return nil
	}
	score := *artifact.PerformanceScore
	entry := core.PerformanceHistoryEntry{
		Timestamp:  scoreHistoryTimestamp(sc),
		Score:      score.Score,
		Signals:    score.Signals,
		Components: append([]core.SlopScoreComponent(nil), score.Components...),
	}
	previous, hasPrevious, err := AppendPerfScoreHistory(path, artifact.ID, entry, rules.ScoreHistoryLimit)
	if hasPrevious {
		previousScore := previous.Score
		delta := score.Score - previousScore
		score.PreviousScore = &previousScore
		score.Delta = &delta
	}
	artifact.PerformanceScore = &score
	return historyWriteSectionDiagnostic("performance", "performance-score", path, err)
}

func recordLegibilityScoreHistory(sc Context, artifact *core.Artifact) *SectionDiagnostic {
	rules := sc.Cfg.Checks.ContextRules
	if !historyToggleEnabled(rules.LegibilityHistory) {
		return nil
	}
	path := LegibilityHistoryPathForBase(sc.Cfg.Cache.Path)
	if path == "" {
		return nil
	}
	score := *artifact.RepoLegibility
	entry := core.LegibilityHistoryEntry{
		Timestamp:  scoreHistoryTimestamp(sc),
		Score:      score.Score,
		Components: append([]core.RepoLegibilityComponent(nil), score.Components...),
	}
	previous, hasPrevious, err := AppendLegibilityHistory(path, artifact.ID, entry, rules.LegibilityHistoryLimit)
	if hasPrevious {
		previousScore := previous.Score
		delta := score.Score - previousScore
		score.PreviousScore = &previousScore
		score.Delta = &delta
	}
	artifact.RepoLegibility = &score
	return historyWriteSectionDiagnostic("context", "legibility", path, err)
}
