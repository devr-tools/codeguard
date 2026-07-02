package support

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/nlrule"
	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type Context struct {
	Cfg         core.Config
	Opts        core.ScanOptions
	Baseline    map[string]core.BaselineEntry
	Diff        map[string]LineRanges
	Artifacts   *ArtifactStore
	RuleStats   *RuleStatsCollector
	Today       time.Time
	RuleCatalog map[string]core.RuleMetadata
	CustomRules []CompiledCustomRule
	NLRuntime   nlrule.Runtime
	Cache       *ScanCache
	// ConfigHash is the conservative all-checks fingerprint used as a fallback
	// when a section has no scoped entry in SectionConfigHash.
	ConfigHash string
	// SectionConfigHash maps a config "family" (see sectionConfigFamily) to a
	// fingerprint of only the settings that can change that family's per-file
	// findings, so editing one section's rules no longer invalidates cached
	// findings for unrelated sections. The "" key holds the all-checks fallback.
	SectionConfigHash map[string]string
	DiffCommand       map[string]diffCommandEnv
	corpus            *fileCorpus
	cleanup           func()
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

func NewContext(ctx context.Context, cfg core.Config, opts core.ScanOptions) (Context, error) {
	customRules, err := compileCustomRules(cfg)
	if err != nil {
		return Context{}, err
	}

	ruleCatalog := config.RuleCatalogForConfig(cfg)
	ensureRuntimeRuleMetadata(ruleCatalog)
	runtime := nlrule.NewRuntime(cfg.AI)

	sc := Context{
		Cfg:               cfg,
		Opts:              opts,
		Artifacts:         NewArtifactStore(),
		RuleStats:         NewRuleStatsCollector(),
		Today:             time.Now(),
		RuleCatalog:       ruleCatalog,
		CustomRules:       customRules,
		NLRuntime:         runtime,
		ConfigHash:        ConfigFingerprint(cfg, runtime.Fingerprint()),
		SectionConfigHash: SectionConfigHashes(cfg, ruleCatalog, runtime.Fingerprint()),
		DiffCommand:       map[string]diffCommandEnv{},
		corpus:            newFileCorpus(),
		cleanup:           func() {},
	}
	if strings.TrimSpace(opts.DiffText) != "" {
		patchedCfg, diffCommand, cleanup, err := MaterializePatchedTargets(ctx, cfg, opts.DiffText)
		if err != nil {
			return Context{}, err
		}
		cfg = patchedCfg
		sc.Cfg = patchedCfg
		sc.DiffCommand = diffCommand
		sc.cleanup = cleanup
		sc.ConfigHash = ConfigFingerprint(patchedCfg, runtime.Fingerprint())
		sc.SectionConfigHash = SectionConfigHashes(patchedCfg, ruleCatalog, runtime.Fingerprint())
	}
	if cfg.Baseline.Path != "" {
		baseline, err := loadBaselineFile(cfg.Baseline.Path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return Context{}, err
		}
		sc.Baseline = baseline
	}
	if strings.TrimSpace(opts.DiffText) == "" && CacheEnabled(cfg.Cache) {
		sc.Cache = LoadScanCache(cfg.Cache.Path)
	}
	if opts.Mode == core.ScanModeDiff {
		var (
			diff map[string]LineRanges
			err  error
		)
		if strings.TrimSpace(opts.DiffText) != "" {
			diff = LoadDiffScopeFromUnifiedDiff(ctx, cfg.Targets, opts.DiffText)
		} else {
			diff, err = LoadDiffScope(ctx, cfg.Targets, opts.BaseRef)
		}
		if err != nil {
			sc.cleanup()
			return Context{}, err
		}
		sc.Diff = diff
	}
	return sc, nil
}

func (sc Context) Close() {
	sc.cleanup()
}

func ensureRuntimeRuleMetadata(catalog map[string]core.RuleMetadata) {
	if _, ok := catalog["design.command-check"]; ok {
		return
	}
	catalog["design.command-check"] = core.NormalizeRuleMetadata(core.RuleMetadata{
		ID:               "design.command-check",
		Section:          "Design Patterns",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelCommandDriven,
		LanguageCoverage: core.ConfigurableRuleLanguageCoverage(),
		Title:            "Language design command",
		Description:      "Fails when a configured language-specific design command exits non-zero.",
		HowToFix:         "Fix the reported issue from the command output or adjust the configured command if it does not fit the target.",
	})
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
	return os.WriteFile(path, append(data, '\n'), 0o600)
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
			// When no source context was available the finding's context
			// fingerprint fell back to the legacy value; drop the duplicate so
			// the entry reads as legacy-only.
			contextFP := finding.ContextFingerprint
			if contextFP == finding.Fingerprint {
				contextFP = ""
			}
			entries = append(entries, core.BaselineEntry{
				Fingerprint:        finding.Fingerprint,
				ContextFingerprint: contextFP,
				RuleID:             finding.RuleID,
				Path:               finding.Path,
				Message:            finding.Message,
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Fingerprint < entries[j].Fingerprint })
	return entries
}

func loadBaselineFile(path string) (map[string]core.BaselineEntry, error) {
	data, err := os.ReadFile(path) //nolint:gosec // operator-supplied baseline file path from config
	if err != nil {
		return nil, err
	}
	var file core.BaselineFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	// Index entries by both fingerprints so IsSuppressed can match either with
	// a single lookup. Legacy-only entries (baseline files written before
	// context fingerprints existed) simply contribute one key.
	out := make(map[string]core.BaselineEntry, len(file.Entries))
	for _, entry := range file.Entries {
		if entry.Fingerprint != "" {
			out[entry.Fingerprint] = entry
		}
		if entry.ContextFingerprint != "" {
			out[entry.ContextFingerprint] = entry
		}
	}
	return out, nil
}
