package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type Runner struct {
	cfg core.Config
}

type scanContext struct {
	cfg         core.Config
	opts        core.ScanOptions
	baseline    map[string]core.BaselineEntry
	diff        map[string]lineRanges
	today       time.Time
	ruleCatalog map[string]core.RuleMetadata
	customRules []compiledCustomRule
	cache       *scanCache
	configHash  string
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

	sc, err := newScanContext(cfg, normalizeScanOptions(opts))
	if err != nil {
		return core.Report{}, err
	}

	report := core.Report{
		Name:        cfg.Name,
		Profile:     cfg.Profile,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Sections:    buildSections(ctx, sc),
	}
	report.Summary = summarizeSections(report.Sections)
	if sc.cache != nil {
		_ = sc.cache.save()
	}
	return report, nil
}

func newScanContext(cfg core.Config, opts core.ScanOptions) (scanContext, error) {
	customRules, err := compileCustomRules(cfg)
	if err != nil {
		return scanContext{}, err
	}

	sc := scanContext{
		cfg:         cfg,
		opts:        opts,
		today:       time.Now(),
		ruleCatalog: config.RuleCatalogForConfig(cfg),
		customRules: customRules,
		configHash:  configFingerprint(cfg),
	}
	if cfg.Baseline.Path != "" {
		baseline, err := loadBaselineFile(cfg.Baseline.Path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return scanContext{}, err
		}
		sc.baseline = baseline
	}
	if cacheEnabled(cfg.Cache) {
		sc.cache = loadScanCache(cfg.Cache.Path)
	}
	if opts.Mode == core.ScanModeDiff {
		diff, err := loadDiffScope(cfg.Targets, opts.BaseRef)
		if err != nil {
			return scanContext{}, err
		}
		sc.diff = diff
	}
	return sc, nil
}

func WriteBaselineFile(path string, entries []core.BaselineEntry) error {
	file := core.BaselineFile{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Entries:     entries,
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func BaselineEntriesFromReport(report core.Report) []core.BaselineEntry {
	var entries []core.BaselineEntry
	seen := map[string]struct{}{}
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if _, ok := seen[finding.Fingerprint]; ok {
				continue
			}
			seen[finding.Fingerprint] = struct{}{}
			entries = append(entries, core.BaselineEntry{
				Fingerprint: finding.Fingerprint,
				RuleID:      finding.RuleID,
				Path:        finding.Path,
				Message:     finding.Message,
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Fingerprint < entries[j].Fingerprint })
	return entries
}

func loadBaselineFile(path string) (map[string]core.BaselineEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file core.BaselineFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	out := make(map[string]core.BaselineEntry, len(file.Entries))
	for _, entry := range file.Entries {
		out[entry.Fingerprint] = entry
	}
	return out, nil
}
