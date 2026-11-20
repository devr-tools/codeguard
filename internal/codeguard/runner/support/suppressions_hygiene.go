package support

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/rules"
)

// UnusedDirectiveDiagnosticID identifies a valid codeguard:ignore directive
// that covered no finding in a full scan.
const UnusedDirectiveDiagnosticID = "suppression.unused-directive"

// MissingReasonDiagnosticID identifies a codeguard:ignore directive that does
// not say why the finding is acceptable.
const MissingReasonDiagnosticID = "suppression.missing-reason"

// directiveUsage records which directives of one file covered a finding. It
// is shared by pointer, so the copy loadInlineSuppressions makes to attach
// peers still reports into the same slice.
type directiveUsage struct {
	mu   sync.Mutex
	used []bool
}

// sectionReadLog records, per check section, the files that section read
// through the corpus. A directive is judged unused only in a file its rule's
// own section actually read: a section that never looked at the file cannot
// have produced the finding the directive was written for.
type sectionReadLog struct {
	mu    sync.Mutex
	reads map[string]map[string]struct{}
}

func sectionReadKey(root string, rel string) string {
	return filepath.Clean(root) + "\x00" + filepath.ToSlash(rel)
}

func normalizedSectionTag(section string) string {
	return strings.ReplaceAll(section, "-", "_")
}

func (log *sectionReadLog) note(section string, root string, rel string) {
	section = normalizedSectionTag(section)
	key := sectionReadKey(root, rel)
	log.mu.Lock()
	defer log.mu.Unlock()
	if log.reads == nil {
		log.reads = map[string]map[string]struct{}{}
	}
	files := log.reads[section]
	if files == nil {
		files = map[string]struct{}{}
		log.reads[section] = files
	}
	files[key] = struct{}{}
}

func (log *sectionReadLog) has(section string, root string, rel string) bool {
	log.mu.Lock()
	defer log.mu.Unlock()
	_, ok := log.reads[normalizedSectionTag(section)][sectionReadKey(root, rel)]
	return ok
}

// files returns every file any section read, sorted, as (root, rel) pairs.
func (log *sectionReadLog) files() [][2]string {
	log.mu.Lock()
	seen := map[string]struct{}{}
	for _, files := range log.reads {
		for key := range files {
			seen[key] = struct{}{}
		}
	}
	log.mu.Unlock()
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([][2]string, 0, len(keys))
	for _, key := range keys {
		root, rel, _ := strings.Cut(key, "\x00")
		out = append(out, [2]string{root, rel})
	}
	return out
}

// directiveReason extracts the explanation from the text that follows a
// directive's rule id (and until date). A comment closer ends it, a leading
// "--" or ":" separator is dropped, and text without a letter is no reason.
func directiveReason(tail string) string {
	for _, closer := range []string{"*/", "-->"} {
		if cut := strings.Index(tail, closer); cut >= 0 {
			tail = tail[:cut]
		}
	}
	tail = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(tail), "-—:;,"))
	if !justificationLetter.MatchString(tail) {
		return ""
	}
	return tail
}

// insideStringLiteral reports whether text, the part of a line before a
// directive, leaves a double-quoted or backtick string open. Single quotes are
// not counted, because apostrophes are common in comment prose.
func insideStringLiteral(text string) bool {
	doubles, backticks := 0, 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '"':
			doubles++
		case '`':
			backticks++
		}
	}
	return doubles%2 == 1 || backticks%2 == 1
}

// tracksDirectiveUsage reports whether this scan judges directives unused.
// Diff and patch scans never do: a directive may cover a finding outside the
// changed lines, which those scans do not report.
func tracksDirectiveUsage(sc Context) bool {
	return sc.corpus != nil &&
		sc.Opts.Mode != core.ScanModeDiff &&
		strings.TrimSpace(sc.Opts.DiffText) == "" &&
		sc.Cfg.Checks.InlineSuppressions.ReportUnusedEnabled()
}

func recordsSectionReads(sc Context) bool {
	return sc.corpus != nil && sc.section != "" &&
		(tracksDirectiveUsage(sc) || sc.Cfg.Checks.InlineSuppressions.RequireReasonEnabled())
}

// markInlineDirectivesUsed records every codeguard:ignore directive that
// covers finding, whether or not it is the one that suppresses it. It runs
// before the confidence policy, the baseline and waivers: a directive whose
// finding is hidden by one of those still describes a real finding, and
// deleting it would resurface the finding as soon as that policy changes.
func markInlineDirectivesUsed(sc Context, finding core.Finding) {
	if !tracksDirectiveUsage(sc) {
		return
	}
	index := inlineSuppressionsFor(sc, finding.Path)
	if index == nil {
		return
	}
	index.markCovering(finding)
	for _, peer := range index.peers {
		peer.markCovering(finding)
	}
}

func (index *inlineSuppressionIndex) markCovering(finding core.Finding) {
	if index.usage == nil {
		return
	}
	var positions []int
	covers := func(position int, requireRange bool) {
		directive := index.directives[position]
		if requireRange && directive.exactLine {
			return
		}
		if rules.MatchRulePattern(directive.ruleID, finding.RuleID) {
			positions = append(positions, position)
		}
	}
	if finding.Line == 0 {
		for position := range index.directives {
			covers(position, true)
		}
	} else {
		for _, position := range index.byLine[finding.Line] {
			covers(position, false)
		}
		for _, position := range index.byLine[finding.Line-1] {
			covers(position, true)
		}
	}
	if len(positions) == 0 {
		return
	}
	index.usage.mu.Lock()
	defer index.usage.mu.Unlock()
	for _, position := range positions {
		index.usage.used[position] = true
	}
}

func (index *inlineSuppressionIndex) usedAt(position int) bool {
	if index.usage == nil {
		return false
	}
	index.usage.mu.Lock()
	defer index.usage.mu.Unlock()
	return index.usage.used[position]
}

// InlineSuppressionHygieneDiagnostics reports, once the scan's sections have
// all finalized, the codeguard:ignore directives that suppressed nothing and
// those that give no reason. Each diagnostic names the report section that
// owns the directive's rule, or none for a selector spanning sections. The
// result is ordered by path, line and rule id.
func InlineSuppressionHygieneDiagnostics(sc Context) []SectionDiagnostic {
	if sc.corpus == nil {
		return nil
	}
	checks := hygieneChecks{
		reportUnused:  tracksDirectiveUsage(sc),
		requireReason: sc.Cfg.Checks.InlineSuppressions.RequireReasonEnabled(),
	}
	if !checks.reportUnused && !checks.requireReason {
		return nil
	}
	var out []SectionDiagnostic
	seenPaths := map[string]struct{}{}
	for _, file := range sc.corpus.sectionReads.files() {
		root, rel := file[0], file[1]
		physical := filepath.Join(root, filepath.FromSlash(rel))
		if _, seen := seenPaths[physical]; seen {
			continue
		}
		seenPaths[physical] = struct{}{}
		index, primary := hygieneIndexFor(sc, root, rel, physical)
		if index == nil {
			continue
		}
		out = append(out, checks.forFile(sc, root, rel, index, primary)...)
	}
	sortHygieneDiagnostics(out)
	return out
}

type hygieneChecks struct {
	reportUnused  bool
	requireReason bool
}

// forFile returns the hygiene diagnostics of one file's native directives.
// Translated gosec directives are golangci-lint's to police, and quoted ones
// are string content.
func (checks hygieneChecks) forFile(sc Context, root string, rel string, index *inlineSuppressionIndex, primary bool) []SectionDiagnostic {
	var out []SectionDiagnostic
	for position, directive := range index.directives {
		if directive.match != "" || directive.quoted {
			continue
		}
		if checks.requireReason && directive.reason == "" && directiveInScope(sc, rel, directive.line) && knownRuleSelector(sc, directive.ruleID) {
			out = append(out, missingReasonDiagnostic(sc, rel, directive))
		}
		if checks.reportUnused && primary && !index.usedAt(position) && directiveEvaluable(sc, directive.ruleID, root, rel) {
			out = append(out, unusedDirectiveDiagnostic(sc, rel, directive))
		}
	}
	return out
}

func sortHygieneDiagnostics(out []SectionDiagnostic) {
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Diagnostic, out[j].Diagnostic
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Metadata["line"] != b.Metadata["line"] {
			return directiveLine(a) < directiveLine(b)
		}
		if a.Metadata["rule"] != b.Metadata["rule"] {
			return a.Metadata["rule"] < b.Metadata["rule"]
		}
		return a.ID < b.ID
	})
}

func directiveLine(diagnostic core.Diagnostic) int {
	line, _ := strconv.Atoi(diagnostic.Metadata["line"])
	return line
}

// hygieneIndexFor returns the directive index of the file at root/rel and
// whether it is the file a finding with path rel resolves to. Usage is only
// recorded on that primary file, so only it can be judged unused; another
// target's file at the same relative path is still checked for reasons.
func hygieneIndexFor(sc Context, root string, rel string, physical string) (*inlineSuppressionIndex, bool) {
	candidates := findingTargetPaths(sc, rel)
	if len(candidates) > 0 && filepath.Clean(candidates[0].path) == filepath.Clean(physical) {
		return inlineSuppressionsFor(sc, rel), true
	}
	return readInlineSuppressions(sc, rel, targetFilePath{root: root, path: physical}), false
}

// directiveInScope limits a diff scan's missing-reason diagnostics to
// directives on changed lines, so a pull request is not told about every
// existing directive in the files it touches.
func directiveInScope(sc Context, rel string, line int) bool {
	if sc.Opts.Mode != core.ScanModeDiff {
		return true
	}
	return matchesDiff(sc, core.Finding{Path: rel, Line: line})
}

// knownRuleSelector reports whether pattern names a rule this scan knows: a
// catalog or custom rule id (or a retired alias of one), a prefix matching at
// least one, or "*". Prose that merely mentions codeguard:ignore parses as a
// directive for a word such as "directive"; it is not reported.
func knownRuleSelector(sc Context, pattern string) bool {
	kind, _ := rules.ClassifyRulePattern(pattern)
	switch kind {
	case rules.RulePatternAll:
		return true
	case rules.RulePatternExact:
		_, ok := sc.RuleCatalog[rules.ResolveRuleAlias(pattern)]
		return ok
	case rules.RulePatternPrefix:
		for id := range sc.RuleCatalog {
			if rules.MatchRulePattern(pattern, id) {
				return true
			}
		}
	}
	return false
}

// directiveEvaluable reports whether this scan can prove a directive unused:
// every rule it selects is known, ran, and was run by its own section over
// this file. A bare "*" never qualifies, and a prefix qualifies only when
// every rule under it does.
func directiveEvaluable(sc Context, pattern string, root string, rel string) bool {
	kind, _ := rules.ClassifyRulePattern(pattern)
	switch kind {
	case rules.RulePatternExact:
		meta, ok := sc.RuleCatalog[rules.ResolveRuleAlias(pattern)]
		return ok && ruleEvaluable(sc, meta, root, rel)
	case rules.RulePatternPrefix:
		matched := false
		for id, meta := range sc.RuleCatalog {
			if !rules.MatchRulePattern(pattern, id) {
				continue
			}
			if !ruleEvaluable(sc, meta, root, rel) {
				return false
			}
			matched = true
		}
		return matched
	default:
		return false
	}
}

// diffOnlyRuleMarkers flag catalog descriptions of rules that report only in
// diff scans; a full scan never runs them, so their directives look unused.
var diffOnlyRuleMarkers = []string{"diff mode", "diff scan", "in diff", "changed line", "base ref", "against the base", "the diff"}

func ruleEvaluable(sc Context, meta core.RuleMetadata, root string, rel string) bool {
	if custom, ok := customRuleByID(sc, meta.ID); ok {
		return !custom.UsesNaturalLanguage() && sc.corpus.sectionReads.has("custom", root, rel)
	}
	// Command-driven rules depend on an external tool or provider that may
	// not have run; disabled rules and sections produce nothing by design.
	if meta.ExecutionModel == core.RuleExecutionModelCommandDriven || !config.RuleEnabled(sc.Cfg, meta) {
		return false
	}
	section := rules.SectionKey(meta)
	if section == "" || section == "change" || section == "contracts" || diffOnlyRule(meta) {
		return false
	}
	if !ruleCoversFileLanguage(meta, rel) {
		return false
	}
	return sc.corpus.sectionReads.has(section, root, rel)
}

func diffOnlyRule(meta core.RuleMetadata) bool {
	description := strings.ToLower(meta.Description)
	for _, marker := range diffOnlyRuleMarkers {
		if strings.Contains(description, marker) {
			return true
		}
	}
	return false
}

var ruleLanguageByExtension = map[string]core.RuleLanguage{
	".go":   core.RuleLanguageGo,
	".py":   core.RuleLanguagePython,
	".ts":   core.RuleLanguageTypeScript,
	".tsx":  core.RuleLanguageTypeScript,
	".mts":  core.RuleLanguageTypeScript,
	".cts":  core.RuleLanguageTypeScript,
	".js":   core.RuleLanguageJavaScript,
	".jsx":  core.RuleLanguageJavaScript,
	".mjs":  core.RuleLanguageJavaScript,
	".cjs":  core.RuleLanguageJavaScript,
	".rs":   core.RuleLanguageRust,
	".java": core.RuleLanguageJava,
	".c":    core.RuleLanguageCPP,
	".cc":   core.RuleLanguageCPP,
	".cpp":  core.RuleLanguageCPP,
	".cxx":  core.RuleLanguageCPP,
	".h":    core.RuleLanguageCPP,
	".hh":   core.RuleLanguageCPP,
	".hpp":  core.RuleLanguageCPP,
	".cs":   core.RuleLanguageCSharp,
	".rb":   core.RuleLanguageRuby,
}

// ruleCoversFileLanguage keeps a language-specific rule's directive from
// being judged in a file of another language, such as a Go rule id quoted in
// a Markdown code sample. Repository-wide and configurable rules apply to any
// file.
func ruleCoversFileLanguage(meta core.RuleMetadata, rel string) bool {
	coverage := meta.LanguageCoverage
	if coverage.Mode != core.RuleLanguageCoverageFixed || len(coverage.Languages) == 0 {
		return true
	}
	language, ok := ruleLanguageByExtension[strings.ToLower(filepath.Ext(rel))]
	if !ok {
		return false
	}
	for _, covered := range coverage.Languages {
		if covered == language {
			return true
		}
	}
	return false
}

func customRuleByID(sc Context, id string) (CompiledCustomRule, bool) {
	for _, rule := range sc.CustomRules {
		if rule.Rule.ID == id {
			return rule, true
		}
	}
	return CompiledCustomRule{}, false
}

// directiveSectionID is the report section that owns the directive's rule,
// or "" when the selector is not one rule.
func directiveSectionID(sc Context, pattern string) string {
	if kind, _ := rules.ClassifyRulePattern(pattern); kind != rules.RulePatternExact {
		return ""
	}
	id := rules.ResolveRuleAlias(pattern)
	if _, ok := customRuleByID(sc, id); ok {
		return "custom"
	}
	meta, ok := sc.RuleCatalog[id]
	if !ok {
		return ""
	}
	return rules.SectionKey(meta)
}

func unusedDirectiveDiagnostic(sc Context, rel string, directive inlineSuppression) SectionDiagnostic {
	return SectionDiagnostic{
		SectionID: directiveSectionID(sc, directive.ruleID),
		Diagnostic: core.Diagnostic{
			ID:       UnusedDirectiveDiagnosticID,
			Level:    "warn",
			Kind:     "configuration",
			Message:  fmt.Sprintf("%s:%d: codeguard:ignore %s suppressed no finding in this scan; remove the directive, or fix its rule id or placement", rel, directive.line, directive.ruleID),
			Path:     rel,
			Metadata: directiveMetadata(directive),
		},
	}
}

func missingReasonDiagnostic(sc Context, rel string, directive inlineSuppression) SectionDiagnostic {
	return SectionDiagnostic{
		SectionID: directiveSectionID(sc, directive.ruleID),
		Diagnostic: core.Diagnostic{
			ID:       MissingReasonDiagnosticID,
			Level:    "warn",
			Kind:     "configuration",
			Message:  fmt.Sprintf("%s:%d: codeguard:ignore %s gives no reason; say why after the rule id, as in \"codeguard:ignore %s -- <reason>\"", rel, directive.line, directive.ruleID, directive.ruleID),
			Path:     rel,
			Metadata: directiveMetadata(directive),
		},
	}
}

func directiveMetadata(directive inlineSuppression) map[string]string {
	metadata := map[string]string{
		"rule":   directive.ruleID,
		"source": "inline",
		"line":   strconv.Itoa(directive.line),
	}
	if directive.expires != "" {
		metadata["until"] = directive.expires
	}
	return metadata
}
