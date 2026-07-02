package runner

import (
	"context"
	"time"

	aitriage "github.com/devr-tools/codeguard/internal/codeguard/ai/triage"
	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnerchecks "github.com/devr-tools/codeguard/internal/codeguard/runner/checks"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

type Runner struct {
	cfg core.Config
}

func New(cfg core.Config) *Runner {
	config.ApplyDefaults(&cfg)
	return &Runner{cfg: cfg}
}

func (r *Runner) Run(ctx context.Context) (core.Report, error) {
	return Run(ctx, r.cfg)
}

func Run(ctx context.Context, cfg core.Config) (core.Report, error) {
	return RunWithOptions(ctx, cfg, core.ScanOptions{Mode: core.ScanModeFull})
}

func RunWithOptions(ctx context.Context, cfg core.Config, opts core.ScanOptions) (core.Report, error) {
	config.ApplyDefaults(&cfg)
	if err := config.Validate(cfg); err != nil {
		return core.Report{}, err
	}

	sc, err := runnersupport.NewContext(ctx, cfg, runnersupport.NormalizeScanOptions(opts))
	if err != nil {
		return core.Report{}, err
	}
	defer sc.Close()

	sections := runnerchecks.Build(ctx, sc)
	sections, triageArtifact := aitriage.Apply(ctx, sc.Cfg, sc.Opts, sections, sc.Cache)

	report := core.Report{
		Name:        sc.Cfg.Name,
		Profile:     sc.Cfg.Profile,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Sections:    sections,
	}
	if triageArtifact != nil {
		sc.Artifacts.Put(*triageArtifact)
	}
	if ruleStats := sc.RuleStats.Snapshot(); len(ruleStats) > 0 {
		sc.Artifacts.Put(core.NewRuleStatsArtifact(ruleStats))
		runnersupport.RecordRuleStatsHistory(sc, ruleStats)
	}
	report.Artifacts = sc.Artifacts.List()
	report.Summary = runnersupport.SummarizeSections(report.Sections)
	if sc.Cache != nil {
		_ = sc.Cache.Save()
	}
	return report, nil
}

func WriteBaselineFile(path string, entries []core.BaselineEntry) error {
	return runnersupport.WriteBaselineFile(path, entries)
}

// SlopHistoryPath derives the slop-score history file path for a config.
func SlopHistoryPath(cfg core.Config) string {
	config.ApplyDefaults(&cfg)
	return runnersupport.SlopHistoryPathForBase(cfg.Cache.Path)
}

// LoadSlopHistory reads the persisted slop-score trend, keyed by artifact ID.
func LoadSlopHistory(path string) map[string][]core.SlopHistoryEntry {
	return runnersupport.LoadSlopHistory(path)
}

// RuleStatsHistoryPath derives the rule-stats history file path for a config.
func RuleStatsHistoryPath(cfg core.Config) string {
	config.ApplyDefaults(&cfg)
	return runnersupport.RuleStatsHistoryPathForBase(cfg.Cache.Path)
}

// LoadRuleStatsHistory reads the persisted per-scan rule suppression stats,
// oldest first.
func LoadRuleStatsHistory(path string) []core.RuleStatsHistoryEntry {
	return runnersupport.LoadRuleStatsHistory(path)
}

func BaselineEntriesFromReport(report core.Report) []core.BaselineEntry {
	return runnersupport.BaselineEntriesFromReport(report)
}
