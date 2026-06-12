package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	internalfix "github.com/devr-tools/codeguard/internal/codeguard/ai/fix"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func runFix(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("fix", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config file or directory path")
	mode := fs.String("mode", string(service.ScanModeFull), "scan mode: full or diff")
	baseRef := fs.String("base-ref", "main", "base branch/ref for diff mode")
	profile := fs.String("profile", "", "optional policy profile override")
	enableAI := fs.Bool("ai", false, "enable optional AI-assisted analysis and fix generation")
	ruleID := fs.String("rule", "", "optional rule id to target")
	path := fs.String("path", "", "optional relative path to target")
	line := fs.Int("line", 0, "optional 1-based line to target")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !*enableAI {
		_, _ = fmt.Fprintln(stderr, "fix requires -ai so unverified AI patch generation is never implicit")
		return 1
	}
	scanMode, err := parseScanMode(*mode)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	cfg, err := loadConfigWithProfile(*configPath, *profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	report, err := service.RunWithOptions(context.Background(), cfg, service.ScanOptions{
		Mode:     scanMode,
		BaseRef:  strings.TrimSpace(*baseRef),
		EnableAI: true,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "scan failed: %v\n", err)
		return 1
	}
	finding, ok := selectFixFinding(report, strings.TrimSpace(*ruleID), strings.TrimSpace(*path), *line)
	if !ok {
		_, _ = fmt.Fprintln(stderr, "no matching finding available for fix generation")
		return 1
	}
	generator, available, err := internalfix.NewAIGenerator(cfg.AI)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "initialize ai generator: %v\n", err)
		return 1
	}
	if !available {
		_, _ = fmt.Fprintln(stderr, "no AI provider is configured for fix generation")
		return 1
	}
	result, err := service.GenerateVerifiedFix(context.Background(), cfg, finding, firstNonEmpty(finding.Why, finding.Message), generator, service.FixOptions{
		BaseRef:      strings.TrimSpace(*baseRef),
		TestCommands: fixVerificationCommands(cfg),
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate verified fix: %v\n", err)
		return 1
	}
	return writeFixResult(stdout, stderr, result, strings.TrimSpace(*format))
}

func fixVerificationCommands(cfg service.Config) []service.FixVerificationCommand {
	out := make([]service.FixVerificationCommand, 0, len(cfg.AI.AutoFix.TestCommands))
	for _, check := range cfg.AI.AutoFix.TestCommands {
		out = append(out, service.FixVerificationCommand{Check: check})
	}
	return out
}

func selectFixFinding(report service.Report, ruleID string, path string, line int) (service.Finding, bool) {
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if ruleID != "" && finding.RuleID != ruleID {
				continue
			}
			if path != "" && finding.Path != path {
				continue
			}
			if line > 0 && finding.Line != line {
				continue
			}
			return finding, true
		}
	}
	return service.Finding{}, false
}

func writeFixResult(stdout io.Writer, stderr io.Writer, result service.VerifiedFix, format string) int {
	switch format {
	case "", "text":
		_, _ = fmt.Fprintf(stdout, "Verified fix: %s\n\n%s\n", firstNonEmpty(result.Summary, "verified patch"), result.Diff)
		if len(result.TestResults) > 0 {
			_, _ = fmt.Fprintln(stdout, "\nVerification:")
			for _, step := range result.TestResults {
				_, _ = fmt.Fprintf(stdout, "- %s (%s)\n", firstNonEmpty(step.CheckName, step.Command), step.TargetName)
			}
		}
		return 0
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "marshal fix result: %v\n", err)
			return 1
		}
		_, _ = stdout.Write(append(data, '\n'))
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unsupported fix output format %q\n", format)
		return 1
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
