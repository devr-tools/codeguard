package rulepacktest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/runner"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// RuleResult compares one rule's expected and reported lines in one file.
type RuleResult struct {
	RuleID string `json:"rule_id"`
	// Expected are the lines "ruleid" annotations name; OK the lines "ok"
	// annotations name; Reported the lines the rule actually reported on.
	// Line 0 is a file-level finding or annotation.
	Expected   []int `json:"expected"`
	OK         []int `json:"ok,omitempty"`
	Reported   []int `json:"reported"`
	Missing    []int `json:"missing,omitempty"`
	Unexpected []int `json:"unexpected,omitempty"`
	// Note explains a rule that cannot be compared (unknown or skipped).
	Note string `json:"note,omitempty"`
}

// FileResult is the outcome for one fixture file.
type FileResult struct {
	Path   string       `json:"path"`
	Passed bool         `json:"passed"`
	Rules  []RuleResult `json:"rules"`
}

// Report is the outcome of a whole run.
type Report struct {
	Files []FileResult `json:"files"`
	// Skipped lists the natural-language rules that were not run, because
	// they need an AI runtime.
	Skipped []string `json:"skipped_rules,omitempty"`
	Passed  int      `json:"passed"`
	Failed  int      `json:"failed"`
}

// OK reports whether every fixture passed.
func (r Report) OK() bool {
	return r.Failed == 0 && len(r.Files) > 0
}

// ErrNoRulePacks is returned when the config defines no custom rules.
var ErrNoRulePacks = errors.New("the config defines no custom rule pack rules (rule_packs)")

// ErrNoAnnotations is returned when no fixture holds an annotation or a
// finding, so a typo in the fixture path cannot pass silently.
var ErrNoAnnotations = errors.New("no fixture file holds a ruleid: or ok: annotation")

// Run scans each fixture path with cfg's custom rule packs and compares the
// findings with the fixtures' annotations. A directory is scanned as its own
// target, so rule path filters match paths relative to it; a file is scanned
// relative to its parent directory. Built-in checks do not run, and
// natural-language rules are skipped rather than sent to a provider.
func Run(ctx context.Context, cfg core.Config, paths []string) (Report, error) {
	packs, skipped := testableRulePacks(cfg.RulePacks)
	if len(packs) == 0 && len(skipped) == 0 {
		return Report{}, ErrNoRulePacks
	}
	known := map[string]bool{}
	for _, pack := range packs {
		for _, rule := range pack.Rules {
			known[rule.ID] = true
		}
	}
	report := Report{Skipped: skipped}
	for _, path := range paths {
		files, err := testPath(ctx, cfg, packs, path)
		if err != nil {
			return Report{}, err
		}
		for _, file := range files {
			file = compareFile(file, known, skipped)
			if len(file.result.Rules) == 0 {
				continue
			}
			report.Files = append(report.Files, file.result)
		}
	}
	if len(report.Files) == 0 {
		return report, ErrNoAnnotations
	}
	for _, file := range report.Files {
		if file.Passed {
			report.Passed++
		} else {
			report.Failed++
		}
	}
	return report, nil
}

// testableRulePacks drops natural-language rules, which need an AI runtime,
// and returns their ids.
func testableRulePacks(packs []core.RulePackConfig) ([]core.RulePackConfig, []string) {
	var out []core.RulePackConfig
	var skipped []string
	for _, pack := range packs {
		kept := pack
		kept.Rules = nil
		for _, rule := range pack.Rules {
			if strings.TrimSpace(rule.NaturalLanguage) != "" {
				skipped = append(skipped, rule.ID)
				continue
			}
			kept.Rules = append(kept.Rules, rule)
		}
		if len(kept.Rules) > 0 {
			out = append(out, kept)
		}
	}
	sort.Strings(skipped)
	return out, skipped
}

type fixtureFile struct {
	display     string
	annotations []Annotation
	reported    map[string][]int
	result      FileResult
}

// testPath scans one fixture path and returns its files with their
// annotations and the findings reported in each.
func testPath(ctx context.Context, cfg core.Config, packs []core.RulePackConfig, path string) ([]fixtureFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	root, only := path, ""
	if !info.IsDir() {
		root, only = filepath.Dir(path), filepath.Base(path)
	}
	rels, err := fixtureFiles(root, only)
	if err != nil {
		return nil, err
	}
	reported := map[string]map[string][]int{}
	if len(packs) > 0 {
		reported, err = scanFixtures(ctx, cfg, packs, root)
		if err != nil {
			return nil, err
		}
	}
	out := make([]fixtureFile, 0, len(rels))
	for _, rel := range rels {
		data, err := readFixture(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		if runnersupport.LooksBinary(data) {
			continue
		}
		out = append(out, fixtureFile{
			display:     filepath.ToSlash(filepath.Join(root, filepath.FromSlash(rel))),
			annotations: ParseAnnotations(string(data)),
			reported:    reported[rel],
		})
	}
	return out, nil
}

func fixtureFiles(root string, only string) ([]string, error) {
	if only != "" {
		return []string{only}, nil
	}
	return runnersupport.WalkFilesWithOptions(root, nil, runnersupport.FileWalkOptions{}, func(string) bool { return true })
}

// readFixture reads one fixture the operator named on the command line.
func readFixture(path string) ([]byte, error) {
	return os.ReadFile(path) //nolint:gosec // fixture path named by the operator on the command line
}

// scanFixtures runs only the custom rules over root and returns, per
// root-relative path, the lines each rule reported on.
func scanFixtures(ctx context.Context, cfg core.Config, packs []core.RulePackConfig, root string) (map[string]map[string][]int, error) {
	off := false
	scanCfg := core.Config{
		Name:             "rules-test",
		Targets:          []core.TargetConfig{{Name: "fixtures", Path: root, Language: fixtureLanguage(cfg)}},
		RulePacks:        packs,
		RespectGitignore: &off,
		Cache:            core.CacheConfig{Enabled: &off},
	}
	scanCfg.Checks.Context = &off
	scanCfg.Checks.InlineSuppressions = core.InlineSuppressionsConfig{ReportUnused: &off, RequireReason: &off}
	report, err := runner.RunWithOptions(ctx, scanCfg, core.ScanOptions{Mode: core.ScanModeFull})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", root, err)
	}
	out := map[string]map[string][]int{}
	for _, section := range report.Sections {
		if section.ID != "custom" {
			continue
		}
		for _, finding := range section.Findings {
			byRule := out[finding.Path]
			if byRule == nil {
				byRule = map[string][]int{}
				out[finding.Path] = byRule
			}
			byRule[finding.RuleID] = append(byRule[finding.RuleID], finding.Line)
		}
	}
	return out, nil
}

// fixtureLanguage keeps the target language valid for config validation; the
// custom section scans every file whatever the language.
func fixtureLanguage(cfg core.Config) string {
	if len(cfg.Targets) > 0 && strings.TrimSpace(cfg.Targets[0].Language) != "" {
		return cfg.Targets[0].Language
	}
	return "go"
}

// compareFile fills file.result from its annotations and findings. Every
// reported line needs a "ruleid" annotation and every "ruleid" annotation a
// reported line; "ok" lines must stay silent. A rule id that no pack defines
// fails the file; a skipped natural-language rule is only noted.
func compareFile(file fixtureFile, known map[string]bool, skipped []string) fixtureFile {
	byRule := map[string]*RuleResult{}
	get := func(id string) *RuleResult {
		if byRule[id] == nil {
			byRule[id] = &RuleResult{RuleID: id}
		}
		return byRule[id]
	}
	for _, annotation := range file.annotations {
		result := get(annotation.RuleID)
		if annotation.Kind == KindRuleID {
			result.Expected = append(result.Expected, annotation.Line)
		} else {
			result.OK = append(result.OK, annotation.Line)
		}
	}
	// An annotation comment names the rule id, which a content regex can
	// match; the annotation is not fixture code, so a finding on it is ignored.
	annotationLines := map[int]bool{}
	for _, annotation := range file.annotations {
		annotationLines[annotation.SourceLine] = true
	}
	for id, lines := range file.reported {
		for _, line := range lines {
			if !annotationLines[line] {
				get(id).Reported = append(get(id).Reported, line)
			}
		}
	}
	file.result = FileResult{Path: file.display, Passed: true}
	ids := make([]string, 0, len(byRule))
	for id := range byRule {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		result := byRule[id]
		evaluateRule(result, known, skipped)
		if (result.Note == "" && (len(result.Missing) > 0 || len(result.Unexpected) > 0)) || result.Note == noteUnknownRule {
			file.result.Passed = false
		}
		file.result.Rules = append(file.result.Rules, *result)
	}
	return file
}

const (
	noteUnknownRule = "unknown rule id: no configured rule pack defines it"
	noteSkippedRule = "skipped: natural-language rule needs an AI runtime and is not run by rules test"
)

func evaluateRule(result *RuleResult, known map[string]bool, skipped []string) {
	result.Expected = sortedUnique(result.Expected)
	result.OK = sortedUnique(result.OK)
	result.Reported = sortedUnique(result.Reported)
	if result.Expected == nil {
		result.Expected = []int{}
	}
	if result.Reported == nil {
		result.Reported = []int{}
	}
	switch {
	case containsString(skipped, result.RuleID):
		result.Note = noteSkippedRule
		return
	case !known[result.RuleID]:
		result.Note = noteUnknownRule
		return
	}
	result.Missing = difference(result.Expected, result.Reported)
	result.Unexpected = difference(result.Reported, result.Expected)
}

func sortedUnique(lines []int) []int {
	if len(lines) == 0 {
		return nil
	}
	sort.Ints(lines)
	out := lines[:1]
	for _, line := range lines[1:] {
		if line != out[len(out)-1] {
			out = append(out, line)
		}
	}
	return out
}

func difference(a []int, b []int) []int {
	set := make(map[int]bool, len(b))
	for _, line := range b {
		set[line] = true
	}
	var out []int
	for _, line := range a {
		if !set[line] {
			out = append(out, line)
		}
	}
	return out
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
