package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// runOWASP prints an OWASP Top 10 (2021) coverage report mapping codeguard's
// security rules to each category and highlighting categories with no rule.
func runOWASP(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("owasp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "optional config path to include custom rule packs")
	profile := fs.String("profile", "", "optional policy profile override")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return exitError
	}

	coverage := service.OWASPCoverage()
	if strings.TrimSpace(*configPath) != "" {
		cfg, ok := loadConfigOrFail(*configPath, *profile, stderr)
		if !ok {
			return exitError
		}
		coverage = service.OWASPCoverageForConfig(cfg)
	}

	switch strings.TrimSpace(*format) {
	case "", "text":
		writeOWASPText(stdout, coverage)
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(coverage); err != nil {
			_, _ = fmt.Fprintf(stderr, "write owasp output: %v\n", err)
			return exitError
		}
	default:
		_, _ = fmt.Fprintf(stderr, "invalid owasp format %q\n", *format)
		return exitError
	}
	return exitOK
}

func writeOWASPText(stdout io.Writer, coverage []service.OWASPCoverageEntry) {
	covered := 0
	for _, entry := range coverage {
		if entry.Covered {
			covered++
		}
	}
	_, _ = fmt.Fprintf(stdout, "OWASP Top 10 (2021) coverage: %d/%d categories have rules\n\n", covered, len(coverage))
	for _, entry := range coverage {
		marker := "gap "
		if entry.Covered {
			marker = "ok  "
		}
		_, _ = fmt.Fprintf(stdout, "[%s] %s (%d rules)\n", marker, entry.Category, len(entry.RuleIDs))
		for _, id := range entry.RuleIDs {
			_, _ = fmt.Fprintf(stdout, "        - %s\n", id)
		}
	}
}
