package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func runBaseline(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		return runBaselineCreate(args, stdout, stderr)
	}
	switch args[0] {
	case "audit":
		return runBaselineAudit(args[1:], stdout, stderr)
	case "prune":
		return runBaselinePrune(args[1:], stdout, stderr)
	case "policy":
		return runBaselinePolicy(args[1:], stdout, stderr)
	default:
		return runBaselineCreate(args, stdout, stderr)
	}
}

type governanceInputs struct {
	cfg          service.Config
	baselinePath string
	format       string
}

func parseGovernanceInputs(command string, args []string, stderr io.Writer) (governanceInputs, bool) {
	fs := flag.NewFlagSet("baseline "+command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := registerScanRunFlags(fs)
	baselinePath := fs.String("baseline", "", "existing baseline path (defaults to baseline.path from config)")
	format := fs.String("format", "text", "output format: text or json")
	if ok, _ := parseFlags(fs, args, stderr); !ok {
		return governanceInputs{}, false
	}
	flags.applyTrustPolicy()
	cfg, ok := loadConfigOrFail(*flags.configPath, *flags.profile, stderr)
	if !ok {
		return governanceInputs{}, false
	}
	if err := applyConfigOverrides(&cfg, *flags.overrides); err != nil {
		_, _ = fmt.Fprintf(stderr, "invalid config override: %v\n", err)
		return governanceInputs{}, false
	}
	path := strings.TrimSpace(*baselinePath)
	if path == "" {
		path = cfg.Baseline.Path
	}
	if path == "" {
		_, _ = fmt.Fprintln(stderr, "baseline path is required through -baseline or baseline.path")
		return governanceInputs{}, false
	}
	if *format != "text" && *format != "json" {
		_, _ = fmt.Fprintln(stderr, "format must be text or json")
		return governanceInputs{}, false
	}
	return governanceInputs{cfg: cfg, baselinePath: path, format: *format}, true
}

// governanceFlagArgs parses command-specific flags without making flag order
// significant. The standard flag package stops at positional arguments, so
// each subcommand owns all of its flags directly instead of sharing a parent.
func runBaselineAudit(args []string, stdout io.Writer, stderr io.Writer) int {
	inputs, ok := parseGovernanceInputs("audit", args, stderr)
	if !ok {
		return exitError
	}
	result, err := auditCurrentBaseline(inputs)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "baseline audit: %v\n", err)
		return exitError
	}
	if err := writeAudit(stdout, result, inputs.format); err != nil {
		_, _ = fmt.Fprintf(stderr, "write audit: %v\n", err)
		return exitError
	}
	return exitOK
}

func runBaselinePrune(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("baseline prune", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := registerScanRunFlags(fs)
	baselinePath := fs.String("baseline", "", "existing baseline path")
	format := fs.String("format", "text", "output format: text or json")
	check := fs.Bool("check", false, "check for stale, invalid, or duplicate entries without writing")
	write := fs.Bool("write", false, "write a stale-only pruned baseline")
	output := fs.String("output", "", "candidate output path (defaults to replacing the source with -write)")
	allowInvalid := fs.Bool("allow-invalid-entries", false, "preserve invalid entries during an explicitly reviewed write")
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}
	if *check == *write {
		_, _ = fmt.Fprintln(stderr, "exactly one of -check or -write is required")
		return exitError
	}
	flags.applyTrustPolicy()
	cfg, ok := loadConfigOrFail(*flags.configPath, *flags.profile, stderr)
	if !ok {
		return exitError
	}
	if err := applyConfigOverrides(&cfg, *flags.overrides); err != nil {
		_, _ = fmt.Fprintf(stderr, "invalid config override: %v\n", err)
		return exitError
	}
	path := strings.TrimSpace(*baselinePath)
	if path == "" {
		path = cfg.Baseline.Path
	}
	if path == "" {
		_, _ = fmt.Fprintln(stderr, "baseline path is required")
		return exitError
	}
	inputs := governanceInputs{cfg: cfg, baselinePath: path, format: *format}
	result, err := auditCurrentBaseline(inputs)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "baseline prune: %v\n", err)
		return exitError
	}
	if err := writeAudit(stdout, result, inputs.format); err != nil {
		_, _ = fmt.Fprintf(stderr, "write audit: %v\n", err)
		return exitError
	}
	if *check {
		if result.Counts.Stale > 0 || result.Counts.Invalid > 0 || len(result.Duplicates) > 0 {
			return exitError
		}
		return exitOK
	}
	if err := WritePruned(path, strings.TrimSpace(*output), result, PruneOptions{AllowInvalid: *allowInvalid}); err != nil {
		_, _ = fmt.Fprintf(stderr, "write pruned baseline: %v\n", err)
		return exitError
	}
	return exitOK
}

func runBaselinePolicy(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("baseline policy", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config file or directory path")
	profile := fs.String("profile", "", "optional policy profile override")
	baselinePath := fs.String("baseline", "", "current baseline path")
	comparisonPath := fs.String("compare-baseline", "", "base-branch baseline path")
	format := fs.String("format", "text", "output format: text or json")
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}
	cfg, ok := loadConfigOrFail(*configPath, *profile, stderr)
	if !ok {
		return exitError
	}
	currentPath := strings.TrimSpace(*baselinePath)
	if currentPath == "" {
		currentPath = cfg.Baseline.Path
	}
	if currentPath == "" || strings.TrimSpace(*comparisonPath) == "" {
		_, _ = fmt.Fprintln(stderr, "-baseline/baseline.path and -compare-baseline are required")
		return exitError
	}
	current, err := Load(currentPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load current baseline: %v\n", err)
		return exitError
	}
	comparison, err := Load(*comparisonPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load comparison baseline: %v\n", err)
		return exitError
	}
	result := ComparePolicy(current, comparison, cfg.Baseline.Governance)
	if *format == "json" {
		err = writeJSONValue(stdout, result)
	} else {
		_, err = fmt.Fprintf(stdout, "added=%d removed=%d violations=%d\n", len(result.Added), len(result.Removed), len(result.Violations))
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "write policy: %v\n", err)
		return exitError
	}
	if len(result.Violations) > 0 {
		return exitError
	}
	return exitOK
}

func auditCurrentBaseline(inputs governanceInputs) (AuditResult, error) {
	file, err := Load(inputs.baselinePath)
	if err != nil {
		return AuditResult{}, err
	}
	cfg := inputs.cfg
	cfg.Baseline.Path = ""
	report, err := service.RunWithOptions(context.Background(), cfg, service.ScanOptions{Mode: service.ScanModeFull, IncludeSuppressed: true})
	if err != nil {
		return AuditResult{}, err
	}
	findings := append([]core.Finding(nil), report.SuppressedFindings...)
	for _, section := range report.Sections {
		findings = append(findings, section.Findings...)
	}
	ownership := make([]OwnershipMapping, 0, len(cfg.Baseline.Governance.Ownership))
	for _, mapping := range cfg.Baseline.Governance.Ownership {
		ownership = append(ownership, OwnershipMapping{Pattern: mapping.Pattern, Owner: mapping.Owner})
	}
	return Audit(file, findings, Options{SampleLimit: cfg.Baseline.Governance.SampleLimit, Ownership: ownership}), nil
}

func writeAudit(w io.Writer, result AuditResult, format string) error {
	if format == "json" {
		return writeJSONValue(w, result)
	}
	_, err := fmt.Fprintf(w, "before=%d active=%d active_exact=%d active_context=%d active_content=%d removed=%d final=%d invalid=%d collisions=%d duplicates=%d\n", result.Counts.Before, result.Counts.Active, result.Counts.ActiveExact, result.Counts.ActiveContext, result.Counts.ActiveContent, result.Counts.Removed, result.Counts.Final, result.Counts.Invalid, len(result.Collisions), len(result.Duplicates))
	return err
}

func writeJSONValue(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
