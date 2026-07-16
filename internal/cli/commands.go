package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func runInit(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", service.DefaultConfigPath(), "output config path")
	interactive := fs.Bool("interactive", false, "prompt for config values in the terminal")
	profile := fs.String("profile", "", "optional policy profile: startup, strict, enterprise, ai-safe")
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}

	cfg, err := exampleConfigForProfile(strings.TrimSpace(*profile))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "init profile: %v\n", err)
		return exitError
	}
	if *interactive {
		if err := promptInitValues(bufio.NewReader(stdin), stdout, output, &cfg.Name); err != nil {
			_, _ = fmt.Fprintf(stderr, "interactive init: %v\n", err)
			return exitError
		}
	}

	if err := service.WriteConfigFile(*output, cfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "write config: %v\n", err)
		return exitError
	}
	_, _ = fmt.Fprintf(stdout, "wrote %s\n", *output)
	return exitOK
}

func runValidate(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config file or directory path")
	profile := fs.String("profile", "", "optional policy profile override")
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}

	cfg, ok := loadConfigOrFail(*configPath, *profile, stderr)
	if !ok {
		return exitError
	}
	if err := service.ValidateConfig(cfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "invalid config: %v\n", err)
		return exitError
	}

	_, _ = fmt.Fprintln(stdout, "config valid")
	return exitOK
}

func runScan(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := registerScanRunFlags(fs)
	inputs := scanInputs{configPath: flags.configPath, mode: flags.mode, baseRef: flags.baseRef}
	format := fs.String("format", "", "optional output format override: text, json, sarif, github")
	enableAI := fs.Bool("ai", false, "enable optional AI-assisted analysis")
	interactive := fs.Bool("interactive", false, "prompt for scan inputs in the terminal")
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}
	flags.applyTrustPolicy()

	if err := promptScanInputs(*interactive, stdin, stdout, &inputs); err != nil {
		_, _ = fmt.Fprintf(stderr, "interactive scan: %v\n", err)
		return exitError
	}

	scanMode, err := parseScanMode(*inputs.mode)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitError
	}

	cfg, ok := loadConfigOrFail(*inputs.configPath, *flags.profile, stderr)
	if !ok {
		return exitError
	}
	if trimmedFormat := strings.TrimSpace(*format); trimmedFormat != "" {
		cfg.Output.Format = trimmedFormat
	}

	if err := executeScan(stdout, cfg, scanMode, strings.TrimSpace(*inputs.baseRef), *enableAI); err != nil {
		_, _ = fmt.Fprintf(stderr, "scan failed: %v\n", err)
		return exitError
	}
	return exitOK
}

func runValidatePatch(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate-patch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config file or directory path")
	format := fs.String("format", "", "optional output format override: text, json, sarif, github")
	enableAI := fs.Bool("ai", false, "enable optional AI-assisted analysis")
	profile := fs.String("profile", "", "optional policy profile override")
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}

	diffText, err := io.ReadAll(stdin)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "read patch stdin: %v\n", err)
		return exitError
	}
	if strings.TrimSpace(string(diffText)) == "" {
		_, _ = fmt.Fprintln(stderr, "validate-patch requires a unified diff on stdin")
		return exitError
	}

	cfg, ok := loadConfigOrFail(*configPath, *profile, stderr)
	if !ok {
		return exitError
	}
	if trimmedFormat := strings.TrimSpace(*format); trimmedFormat != "" {
		cfg.Output.Format = trimmedFormat
	}

	report, err := service.RunWithOptions(context.Background(), cfg, service.ScanOptions{
		Mode:     service.ScanModeDiff,
		BaseRef:  "stdin",
		DiffText: string(diffText),
		EnableAI: *enableAI,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "patch validation failed: %v\n", err)
		return exitError
	}
	if err := service.WriteReport(stdout, report, cfg.Output.Format); err != nil {
		_, _ = fmt.Fprintf(stderr, "write report: %v\n", err)
		return exitError
	}
	if report.Summary.FailedSections > 0 {
		return exitError
	}
	return exitOK
}

func runBaseline(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("baseline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := registerScanRunFlags(fs)
	outputPath := fs.String("output", "codeguard-baseline.json", "baseline output path")
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}
	flags.applyTrustPolicy()

	cfg, ok := loadConfigOrFail(*flags.configPath, *flags.profile, stderr)
	if !ok {
		return exitError
	}
	cfg.Baseline.Path = ""

	report, err := service.RunWithOptions(context.Background(), cfg, service.ScanOptions{
		Mode:    service.ScanMode(strings.TrimSpace(*flags.mode)),
		BaseRef: strings.TrimSpace(*flags.baseRef),
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "baseline scan failed: %v\n", err)
		return exitError
	}
	if err := service.WriteBaselineFile(*outputPath, service.BaselineEntriesFromReport(report)); err != nil {
		_, _ = fmt.Fprintf(stderr, "write baseline: %v\n", err)
		return exitError
	}
	_, _ = fmt.Fprintf(stdout, "wrote %s\n", *outputPath)
	return exitOK
}

func promptInitValues(reader *bufio.Reader, stdout io.Writer, output *string, configName *string) error {
	var err error
	*output, err = promptString(reader, stdout, "config output path", *output)
	if err != nil {
		return err
	}
	*configName, err = promptString(reader, stdout, "config name", *configName)
	return err
}

type scanInputs struct {
	configPath *string
	mode       *string
	baseRef    *string
}

func promptScanInputs(interactive bool, stdin io.Reader, stdout io.Writer, inputs *scanInputs) error {
	if !interactive {
		return nil
	}

	reader := bufio.NewReader(stdin)
	var err error
	*inputs.configPath, err = promptString(reader, stdout, "config path", *inputs.configPath)
	if err != nil {
		return err
	}
	*inputs.mode, err = promptString(reader, stdout, "scan mode (full|diff)", *inputs.mode)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*inputs.mode) != string(service.ScanModeDiff) {
		return nil
	}

	*inputs.baseRef, err = promptString(reader, stdout, "base ref", *inputs.baseRef)
	return err
}

func parseScanMode(mode string) (service.ScanMode, error) {
	scanMode := service.ScanMode(strings.TrimSpace(mode))
	if scanMode != service.ScanModeFull && scanMode != service.ScanModeDiff {
		return "", fmt.Errorf("invalid scan mode %q", mode)
	}
	return scanMode, nil
}

func executeScan(stdout io.Writer, cfg service.Config, scanMode service.ScanMode, baseRef string, enableAI bool) error {
	report, err := service.RunWithOptions(context.Background(), cfg, service.ScanOptions{
		Mode:     scanMode,
		BaseRef:  baseRef,
		EnableAI: enableAI,
	})
	if err != nil {
		return err
	}
	if err := writeScanMetadata(stdout, cfg.Output.Format, scanMode, baseRef); err != nil {
		return err
	}
	if err := service.WriteReport(stdout, report, cfg.Output.Format); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	writePerformanceUpgradeHint(stdout, cfg)
	if report.Summary.FailedSections > 0 {
		return fmt.Errorf("one or more sections failed")
	}
	return nil
}

// writePerformanceUpgradeHint nudges configs that predate the performance
// section (checks.performance absent, i.e. nil) to opt in. An explicit
// performance: false is a deliberate choice and stays silent, as do the
// machine-readable output formats.
func writePerformanceUpgradeHint(stdout io.Writer, cfg service.Config) {
	if cfg.Checks.Performance != nil {
		return
	}
	if format := strings.TrimSpace(cfg.Output.Format); format != "" && format != "text" {
		return
	}
	_, _ = fmt.Fprintln(stdout, "\nnote: this config predates the performance check section (N+1 queries, alloc-heavy loops, blocking I/O in handlers, unbounded concurrency).")
	_, _ = fmt.Fprintln(stdout, "      enable it with `performance: true` under `checks:` in your codeguard config, or silence this note with `performance: false`. See docs/checks.md#performance.")
}

func writeScanMetadata(stdout io.Writer, format string, scanMode service.ScanMode, baseRef string) error {
	trimmedFormat := strings.TrimSpace(format)
	if trimmedFormat != "" && trimmedFormat != "text" {
		return nil
	}
	if scanMode != service.ScanModeDiff {
		return nil
	}
	_, err := fmt.Fprintf(stdout, "Base Ref: %s\n", baseRef)
	return err
}
