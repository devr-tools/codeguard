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
	format := fs.String("format", "", "optional output format override: text, json, sarif, github, cyclonedx")
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
	format := fs.String("format", "", "optional output format override: text, json, sarif, github, cyclonedx")
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
