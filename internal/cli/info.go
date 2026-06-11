package cli

import (
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
		_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", rule.ID, rule.DefaultLevel, rule.Section, rule.Title)
	}
	return 0
}

func runExplain(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "optional config path to include custom rule packs")
	profile := fs.String("profile", "", "optional policy profile override")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() == 0 {
		_, _ = fmt.Fprintln(stderr, "explain requires a rule id")
		return 1
	}

	ruleID := fs.Arg(0)
	rule, ok := service.ExplainRule(ruleID)
	if strings.TrimSpace(*configPath) != "" {
		cfg, err := loadConfigWithProfile(*configPath, *profile)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
			return 1
		}
		rule, ok = service.ExplainRuleForConfig(cfg, ruleID)
	}
	if !ok {
		_, _ = fmt.Fprintf(stderr, "unknown rule %q\n", ruleID)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "%s\ntitle: %s\nsection: %s\nlevel: %s\n%s\n", rule.ID, rule.Title, rule.Section, rule.DefaultLevel, rule.Description)
	if strings.TrimSpace(rule.HowToFix) != "" {
		_, _ = fmt.Fprintf(stdout, "how to fix: %s\n", rule.HowToFix)
	}
	return 0
}

func runProfiles(stdout io.Writer) int {
	for _, profile := range service.Profiles() {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\n", profile.Name, profile.Description)
	}
	return 0
}
