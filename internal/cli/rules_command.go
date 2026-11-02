package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/suggest"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// rulesFilter narrows `codeguard rules`. The zero value keeps every rule.
type rulesFilter struct {
	section string
	search  string
	level   string
	enabled bool
}

func runRules(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "optional config path to include custom rule packs (and to resolve -enabled)")
	profile := fs.String("profile", "", "optional policy profile override")
	section := fs.String("section", "", "only rules in this section id (quality, security, supply_chain, ...) or display name")
	search := fs.String("search", "", "only rules whose id, title, or description contains this text (case-insensitive)")
	enabled := fs.Bool("enabled", false, "only rules that run under the resolved config/profile")
	level := fs.String("level", "", "only rules with this default level: fail or warn")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	filter := rulesFilter{section: strings.TrimSpace(*section), search: strings.TrimSpace(*search), level: strings.ToLower(strings.TrimSpace(*level)), enabled: *enabled}
	if err := validateRulesFlags(filter, strings.TrimSpace(*format)); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitError
	}

	rules := service.Rules()
	var ec *explainConfig
	if strings.TrimSpace(*configPath) != "" || filter.enabled {
		resolved, err := resolveExplainConfig(*configPath, *profile, loadConfigWithProfile, func(note string) {
			_, _ = fmt.Fprintln(stderr, note)
		})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
			return exitError
		}
		ec = &resolved
		rules = service.RulesForConfig(resolved.cfg)
	}
	rules = filterRules(rules, filter, ec)

	if strings.TrimSpace(*format) == "json" {
		return writeRulesJSON(stdout, stderr, rules, ec)
	}
	for _, rule := range rules {
		_, _ = fmt.Fprintln(stdout, rulesTextLine(rule))
	}
	return exitOK
}

func validateRulesFlags(filter rulesFilter, format string) error {
	switch format {
	case "", "text", "json":
	default:
		return fmt.Errorf("invalid rules format %q (valid: text, json)", format)
	}
	switch filter.level {
	case "", "fail", "warn":
	default:
		return fmt.Errorf("invalid -level %q (valid: fail, warn)", filter.level)
	}
	if filter.section != "" && !knownRuleSection(filter.section) {
		known := knownRuleSections()
		return fmt.Errorf("unknown -section %q%s (valid: %s)", filter.section, suggest.Hint(filter.section, known), strings.Join(known, ", "))
	}
	return nil
}

// knownRuleSections lists the section ids the built-in catalog uses.
func knownRuleSections() []string {
	seen := map[string]bool{}
	for _, rule := range service.Rules() {
		if key := service.RuleSectionKey(rule); key != "" {
			seen[key] = true
		}
	}
	out := make([]string, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func knownRuleSection(section string) bool {
	normalized := normalizeRulesSectionFilter(section)
	for _, rule := range service.Rules() {
		if matchesSection(rule, normalized) {
			return true
		}
	}
	return normalized == "custom"
}

// normalizeRulesSectionFilter accepts the runner's "supply-chain" spelling and
// any casing alongside the report's section ids.
func normalizeRulesSectionFilter(section string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(section)), "-", "_")
}

func matchesSection(rule service.RuleMetadata, normalized string) bool {
	if service.RuleSectionKey(rule) == normalized {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(rule.Section), normalized) ||
		normalizeRulesSectionFilter(rule.Section) == normalized
}

// inSectionFilter reports whether rule passes the -section filter; custom
// rule-pack rules with a free-form section match "custom".
func inSectionFilter(rule service.RuleMetadata, section string) bool {
	if section == "" || matchesSection(rule, section) {
		return true
	}
	return section == "custom" && service.RuleSectionKey(rule) == ""
}

func filterRules(rules []service.RuleMetadata, filter rulesFilter, ec *explainConfig) []service.RuleMetadata {
	section := normalizeRulesSectionFilter(filter.section)
	search := strings.ToLower(filter.search)
	out := make([]service.RuleMetadata, 0, len(rules))
	for _, rule := range rules {
		if !inSectionFilter(rule, section) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(rule.ID+"\n"+rule.Title+"\n"+rule.Description), search) {
			continue
		}
		if filter.level != "" && !strings.EqualFold(rule.DefaultLevel, filter.level) {
			continue
		}
		if filter.enabled && ec != nil && !service.RuleEnabled(ec.cfg, rule) {
			continue
		}
		out = append(out, rule)
	}
	return out
}

func rulesTextLine(rule service.RuleMetadata) string {
	var line strings.Builder
	line.WriteString(rule.ID)
	line.WriteByte('\t')
	line.WriteString(rule.DefaultLevel)
	line.WriteByte('\t')
	line.WriteString(string(rule.ExecutionModel))
	line.WriteByte('\t')
	line.WriteString(rule.LanguageCoverage.String())
	line.WriteByte('\t')
	line.WriteString(rule.Section)
	line.WriteByte('\t')
	line.WriteString(rule.Title)
	if rule.OWASPCategory != "" {
		line.WriteByte('\t')
		line.WriteString(string(rule.OWASPCategory))
	}
	return line.String()
}

// rulesJSONEntry is one rule in `codeguard rules -format json`: the list
// columns plus the section id, confidence, and (when a config was resolved)
// whether the rule runs under it. `codeguard explain <id> -format json`
// carries the full detail.
type rulesJSONEntry struct {
	ID               string                        `json:"id"`
	Title            string                        `json:"title"`
	Section          string                        `json:"section"`
	SectionKey       string                        `json:"section_key,omitempty"`
	Level            string                        `json:"level"`
	Confidence       string                        `json:"confidence,omitempty"`
	ExecutionModel   string                        `json:"execution_model"`
	LanguageCoverage explainLanguageCoverageOutput `json:"language_coverage"`
	OWASPCategory    string                        `json:"owasp_category,omitempty"`
	Description      string                        `json:"description"`
	HasExamples      bool                          `json:"has_examples"`
	Enabled          *bool                         `json:"enabled,omitempty"`
}

func writeRulesJSON(stdout io.Writer, stderr io.Writer, rules []service.RuleMetadata, ec *explainConfig) int {
	entries := make([]rulesJSONEntry, 0, len(rules))
	for _, rule := range rules {
		entry := rulesJSONEntry{
			ID:             rule.ID,
			Title:          rule.Title,
			Section:        rule.Section,
			SectionKey:     service.RuleSectionKey(rule),
			Level:          rule.DefaultLevel,
			Confidence:     rule.Confidence,
			ExecutionModel: string(rule.ExecutionModel),
			LanguageCoverage: explainLanguageCoverageOutput{
				Mode:      string(rule.LanguageCoverage.Mode),
				Languages: explainLanguages(rule.LanguageCoverage.Languages),
			},
			OWASPCategory: string(rule.OWASPCategory),
			Description:   rule.Description,
			HasExamples:   len(rule.Examples) > 0,
		}
		if ec != nil {
			enabled := service.RuleEnabled(ec.cfg, rule)
			entry.Enabled = &enabled
		}
		entries = append(entries, entry)
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(map[string]any{"rules": entries}); err != nil {
		_, _ = fmt.Fprintf(stderr, "write rules output: %v\n", err)
		return exitError
	}
	return exitOK
}
