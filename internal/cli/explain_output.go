package cli

import (
	"errors"
	"fmt"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// explainConfig is the resolved config `explain` reads current values and
// enablement from, plus a human-readable note on where it came from.
type explainConfig struct {
	cfg    service.Config
	source string
}

// builtInDefaultsSource labels the zero-config fallback in explain output.
const builtInDefaultsSource = "built-in defaults; no codeguard config found"

// resolveExplainConfig loads the config explain describes. A named config
// must load; otherwise a config discovered in the working directory is used,
// falling back to the same built-in defaults `codeguard scan` uses when there
// is none. warn receives a note when a discovered config fails to load.
func resolveExplainConfig(configPath string, profile string, load func(path string, profile string) (service.Config, error), warn func(string)) (explainConfig, error) {
	named := strings.TrimSpace(configPath)
	cfg, err := load(named, profile)
	if err == nil {
		source := named
		if source == "" {
			source = "discovered codeguard config"
			if resolved, resolveErr := service.ResolveConfigPath(""); resolveErr == nil && resolved != "" {
				source = resolved
			}
		}
		return explainConfig{cfg: cfg, source: source}, nil
	}
	if named != "" {
		return explainConfig{}, err
	}
	if !errors.Is(err, service.ErrConfigNotFound) && warn != nil {
		warn(fmt.Sprintf("note: ignoring the discovered codeguard config: %v", err))
	}
	cfg, err = service.DefaultScanConfig(".", profile)
	if err != nil {
		return explainConfig{}, err
	}
	return explainConfig{cfg: cfg, source: builtInDefaultsSource}, nil
}

// explainOutput is the structured form of `codeguard explain`, shared by
// `explain -format json|agent`, the MCP explain tool, and the
// codeguard://rules/<id> resource.
type explainOutput struct {
	ID               string                        `json:"id"`
	Title            string                        `json:"title"`
	Section          string                        `json:"section"`
	SectionKey       string                        `json:"section_key,omitempty"`
	Level            string                        `json:"level"`
	Confidence       string                        `json:"confidence,omitempty"`
	ExecutionModel   string                        `json:"execution_model"`
	LanguageCoverage explainLanguageCoverageOutput `json:"language_coverage"`
	Description      string                        `json:"description"`
	Why              string                        `json:"why"`
	HowToFix         string                        `json:"how_to_fix"`
	FixTemplate      string                        `json:"fix_template"`
	FixTemplateKind  string                        `json:"fix_template_kind,omitempty"`
	OWASPCategory    string                        `json:"owasp_category,omitempty"`
	Examples         []service.RuleExample         `json:"examples"`
	ConfigKeys       []explainConfigKeyOutput      `json:"config_keys"`
	ConfigSource     string                        `json:"config_source,omitempty"`
	Enabled          *bool                         `json:"enabled,omitempty"`
	Suppression      explainSuppressionOutput      `json:"suppression"`
	Profiles         []string                      `json:"profiles"`
	Aliases          []string                      `json:"aliases"`
	Docs             string                        `json:"docs,omitempty"`
	DocsURL          string                        `json:"docs_url,omitempty"`
}

type explainLanguageCoverageOutput struct {
	Mode      string   `json:"mode"`
	Languages []string `json:"languages"`
}

// explainConfigKeyOutput is one controlling key with its value in the
// resolved config. Unset marks a known key whose pointer field is unset.
type explainConfigKeyOutput struct {
	Key   string `json:"key"`
	Role  string `json:"role"`
	Value any    `json:"value,omitempty"`
	Unset bool   `json:"unset,omitempty"`
}

type explainSuppressionOutput struct {
	Inline string `json:"inline"`
	Waiver string `json:"waiver"`
}

func buildExplainOutput(rule service.RuleMetadata, ec *explainConfig) explainOutput {
	out := explainOutput{
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
		Description:     rule.Description,
		Why:             rule.Description,
		HowToFix:        rule.HowToFix,
		FixTemplate:     rule.FixTemplate.Text,
		FixTemplateKind: string(rule.FixTemplate.Kind),
		OWASPCategory:   string(rule.OWASPCategory),
		Examples:        rule.Examples,
		ConfigKeys:      make([]explainConfigKeyOutput, 0, len(rule.ConfigKeys)),
		Suppression: explainSuppressionOutput{
			Inline: inlineSuppressionExample(rule),
			Waiver: waiverYAMLExample(rule),
		},
		Profiles: nonNilStrings(service.RuleProfileNames(rule)),
		Aliases:  nonNilStrings(service.RuleAliases(rule.ID)),
	}
	if out.Examples == nil {
		out.Examples = []service.RuleExample{}
	}
	// Custom rule-pack rules have no row in the rule index to link to.
	if _, builtIn := service.ExplainRule(rule.ID); builtIn {
		out.Docs = service.RuleDocsPath(rule.ID)
		out.DocsURL = service.RuleDocsURL(rule.ID)
	}
	for _, key := range rule.ConfigKeys {
		item := explainConfigKeyOutput{Key: key.Key, Role: string(key.Role)}
		if ec != nil {
			if value, ok := service.LookupConfigKey(ec.cfg, key.Key); ok {
				item.Value = value
				item.Unset = value == nil
			}
		}
		out.ConfigKeys = append(out.ConfigKeys, item)
	}
	if ec != nil {
		out.ConfigSource = ec.source
		enabled := service.RuleEnabled(ec.cfg, rule)
		out.Enabled = &enabled
	}
	return out
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func explainLanguages(languages []service.RuleLanguage) []string {
	if len(languages) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(languages))
	for _, language := range languages {
		out = append(out, string(language))
	}
	return out
}

// inlineSuppressionExample renders the codeguard:ignore directive in the
// comment syntax of the rule's example (or first covered language).
func inlineSuppressionExample(rule service.RuleMetadata) string {
	directive := "codeguard:ignore " + rule.ID + " -- <reason>"
	switch commentStyleFor(rule) {
	case "#":
		return "# " + directive
	case "html":
		return "<!-- " + directive + " -->"
	default:
		return "// " + directive
	}
}

func commentStyleFor(rule service.RuleMetadata) string {
	language := ""
	if len(rule.Examples) > 0 {
		language = rule.Examples[0].Language
	} else if len(rule.LanguageCoverage.Languages) > 0 {
		language = string(rule.LanguageCoverage.Languages[0])
	}
	switch language {
	case "python", "yaml", "ruby", "shell", "toml", "text":
		return "#"
	case "markdown":
		return "html"
	default:
		return "//"
	}
}

// waiverYAMLExample renders a codeguard.yaml waivers entry for the rule.
func waiverYAMLExample(rule service.RuleMetadata) string {
	path := "path/to/file/or/glob/**"
	if len(rule.Examples) > 0 && strings.TrimSpace(rule.Examples[0].Path) != "" {
		path = rule.Examples[0].Path
	}
	return "waivers:\n" +
		"  - rule: " + rule.ID + "\n" +
		"    path: " + path + "\n" +
		"    reason: <why this finding is accepted>\n" +
		"    expires_on: YYYY-MM-DD\n"
}
