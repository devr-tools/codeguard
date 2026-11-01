package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/suggest"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func runExplain(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "config path; defaults to a codeguard config in the working directory, then built-in defaults")
	format := fs.String("format", "text", "output format: text, json (agent is an alias of json)")
	profile := fs.String("profile", "", "optional policy profile override")
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	if fs.NArg() == 0 {
		_, _ = fmt.Fprintln(stderr, "explain requires a rule id")
		return exitError
	}
	outputFormat := strings.TrimSpace(*format)
	switch outputFormat {
	case "", "text", "json", "agent":
	default:
		_, _ = fmt.Fprintf(stderr, "invalid explain format %q (valid: text, json, agent)\n", *format)
		return exitError
	}

	ec, err := resolveExplainConfig(*configPath, *profile, loadConfigWithProfile, func(note string) {
		_, _ = fmt.Fprintln(stderr, note)
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return exitError
	}
	ruleID := fs.Arg(0)
	rule, ok := service.ExplainRuleForConfig(ec.cfg, ruleID)
	if !ok {
		_, _ = fmt.Fprintf(stderr, "unknown rule %q%s\n", ruleID, suggest.Hint(ruleID, ruleIDs(service.RulesForConfig(ec.cfg))))
		_, _ = fmt.Fprintln(stderr, "run `codeguard rules` to list every rule id")
		return exitError
	}

	out := buildExplainOutput(rule, &ec)
	if outputFormat == "" || outputFormat == "text" {
		writeExplainText(stdout, out)
		return exitOK
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		_, _ = fmt.Fprintf(stderr, "write explain output: %v\n", err)
		return exitError
	}
	return exitOK
}

func ruleIDs(rules []service.RuleMetadata) []string {
	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}
	return ids
}

func runProfiles(stdout io.Writer) int {
	for _, profile := range service.Profiles() {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\n", profile.Name, profile.Description)
	}
	return exitOK
}
