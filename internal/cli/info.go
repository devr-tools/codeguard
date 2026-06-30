package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func runRules(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "optional config path to include custom rule packs")
	profile := fs.String("profile", "", "optional policy profile override")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	rules := service.Rules()
	if strings.TrimSpace(*configPath) != "" {
		cfg, err := loadConfigWithProfile(*configPath, *profile)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
			return 1
		}
		rules = service.RulesForConfig(cfg)
	}
	for _, rule := range rules {
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
		_, _ = fmt.Fprintln(stdout, line.String())
	}
	return 0
}

func runExplain(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "optional config path to include custom rule packs")
	format := fs.String("format", "text", "output format: text, agent")
	profile := fs.String("profile", "", "optional policy profile override")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() == 0 {
		_, _ = fmt.Fprintln(stderr, "explain requires a rule id")
		return 1
	}

	ruleID := fs.Arg(0)
	rule, ok, err := resolveExplainRule(*configPath, *profile, ruleID)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	if !ok {
		_, _ = fmt.Fprintf(stderr, "unknown rule %q\n", ruleID)
		return 1
	}

	switch strings.TrimSpace(*format) {
	case "", "text":
		writeExplainText(stdout, rule)
	case "agent":
		if err := writeExplainAgent(stdout, rule); err != nil {
			_, _ = fmt.Fprintf(stderr, "write explain output: %v\n", err)
			return 1
		}
	default:
		_, _ = fmt.Fprintf(stderr, "invalid explain format %q\n", *format)
		return 1
	}
	return 0
}

func resolveExplainRule(configPath string, profile string, ruleID string) (service.RuleMetadata, bool, error) {
	rule, ok := service.ExplainRule(ruleID)
	if strings.TrimSpace(configPath) == "" {
		return rule, ok, nil
	}

	cfg, err := loadConfigWithProfile(configPath, profile)
	if err != nil {
		return service.RuleMetadata{}, false, err
	}
	rule, ok = service.ExplainRuleForConfig(cfg, ruleID)
	return rule, ok, nil
}

func writeExplainText(stdout io.Writer, rule service.RuleMetadata) {
	_, _ = fmt.Fprintf(stdout, "%s\ntitle: %s\nsection: %s\nlevel: %s\nexecution model: %s\nlanguage coverage: %s\n%s\n", rule.ID, rule.Title, rule.Section, rule.DefaultLevel, rule.ExecutionModel, rule.LanguageCoverage, rule.Description)
	if rule.OWASPCategory != "" {
		_, _ = fmt.Fprintf(stdout, "owasp: %s\n", rule.OWASPCategory)
	}
	if strings.TrimSpace(rule.HowToFix) != "" {
		_, _ = fmt.Fprintf(stdout, "how to fix: %s\n", rule.HowToFix)
	}
}

func writeExplainAgent(stdout io.Writer, rule service.RuleMetadata) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(buildExplainAgentOutput(rule))
}

type explainAgentOutput struct {
	ID               string                        `json:"id"`
	Title            string                        `json:"title"`
	Section          string                        `json:"section"`
	Level            string                        `json:"level"`
	ExecutionModel   string                        `json:"execution_model"`
	LanguageCoverage explainLanguageCoverageOutput `json:"language_coverage"`
	Description      string                        `json:"description"`
	Why              string                        `json:"why"`
	HowToFix         string                        `json:"how_to_fix"`
	FixTemplate      string                        `json:"fix_template"`
	OWASPCategory    string                        `json:"owasp_category,omitempty"`
}

type explainLanguageCoverageOutput struct {
	Mode      string   `json:"mode"`
	Languages []string `json:"languages"`
}

func buildExplainAgentOutput(rule service.RuleMetadata) explainAgentOutput {
	return explainAgentOutput{
		ID:             rule.ID,
		Title:          rule.Title,
		Section:        rule.Section,
		Level:          rule.DefaultLevel,
		ExecutionModel: string(rule.ExecutionModel),
		LanguageCoverage: explainLanguageCoverageOutput{
			Mode:      string(rule.LanguageCoverage.Mode),
			Languages: explainLanguages(rule.LanguageCoverage.Languages),
		},
		Description:   rule.Description,
		Why:           rule.Description,
		HowToFix:      rule.HowToFix,
		FixTemplate:   rule.FixTemplate,
		OWASPCategory: string(rule.OWASPCategory),
	}
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

func runProfiles(stdout io.Writer) int {
	for _, profile := range service.Profiles() {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\n", profile.Name, profile.Description)
	}
	return 0
}
