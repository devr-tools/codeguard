package support

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type Context struct {
	Cfg         core.Config
	Opts        core.ScanOptions
	Baseline    map[string]core.BaselineEntry
	Diff        map[string]LineRanges
	Today       time.Time
	RuleCatalog map[string]core.RuleMetadata
	CustomRules []CompiledCustomRule
	Cache       *ScanCache
	ConfigHash  string
}

func NormalizeScanOptions(opts core.ScanOptions) core.ScanOptions {
	if opts.Mode == "" {
		opts.Mode = core.ScanModeFull
	}
	if opts.BaseRef == "" {
		opts.BaseRef = "main"
	}
	return opts
}

func NewContext(cfg core.Config, opts core.ScanOptions) (Context, error) {
	customRules, err := compileCustomRules(cfg)
	if err != nil {
		return Context{}, err
	}

	sc := Context{
		Cfg:         cfg,
		Opts:        opts,
		Today:       time.Now(),
		RuleCatalog: config.RuleCatalogForConfig(cfg),
		CustomRules: customRules,
		ConfigHash:  ConfigFingerprint(cfg),
	}
	if cfg.Baseline.Path != "" {
		baseline, err := loadBaselineFile(cfg.Baseline.Path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return Context{}, err
		}
		sc.Baseline = baseline
	}
	if CacheEnabled(cfg.Cache) {
		sc.Cache = LoadScanCache(cfg.Cache.Path)
	}
	if opts.Mode == core.ScanModeDiff {
		diff, err := LoadDiffScope(cfg.Targets, opts.BaseRef)
		if err != nil {
			return Context{}, err
		}
		sc.Diff = diff
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
