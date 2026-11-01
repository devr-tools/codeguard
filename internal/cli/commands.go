package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/initconfig"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func runInit(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", service.DefaultConfigPath(), "output config path")
	interactive := fs.Bool("interactive", false, "prompt for config values in the terminal")
	profile := fs.String("profile", "", "optional policy profile: startup, strict, enterprise, ai-safe")
	root := fs.String("path", ".", "repository root to detect languages, entrypoints, workflows and manifests from")
	full := fs.Bool("full", false, "write every setting with its resolved default instead of the small detected config")
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}

	layout, err := initconfig.Detect(*root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "init: detect repository layout: %v\n", err)
		return exitError
	}
	name := ""
	if *interactive {
		name = "codeguard-default"
		if promptErr := promptInitValues(bufio.NewReader(stdin), stdout, output, &name); promptErr != nil {
			_, _ = fmt.Fprintf(stderr, "interactive init: %v\n", promptErr)
			return exitError
		}
	}
	targetPath, err := initconfig.TargetPathForConfig(*output, *root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "init: %v\n", err)
		return exitError
	}
	cfg, err := initconfig.Build(layout, initconfig.Options{Profile: *profile, TargetPath: targetPath, Name: name})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "init profile: %v\n", err)
		return exitError
	}

	if err = writeInitConfig(*output, cfg, layout, *full); err != nil {
		_, _ = fmt.Fprintf(stderr, "write config: %v\n", err)
		return exitError
	}
	for _, line := range initconfig.DescribeLayout(layout) {
		_, _ = fmt.Fprintln(stdout, line)
	}
	_, _ = fmt.Fprintf(stdout, "wrote %s\n", *output)
	_, _ = fmt.Fprintf(stdout, "next: run `codeguard scan`; every setting not in the file uses its default (see docs/checks.md)\n")
	return exitOK
}

// writeInitConfig writes the small detected config, or with full the same
// effective config expanded to every key and its resolved default.
func writeInitConfig(output string, cfg service.Config, layout initconfig.Layout, full bool) error {
	if full {
		return service.WriteConfigFile(output, cfg)
	}
	data, err := initconfig.Render(output, cfg, layout)
	if err != nil {
		return err
	}
	return initconfig.WriteFile(output, data)
}

func runValidate(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config file or directory path")
	profile := fs.String("profile", "", "optional policy profile override")
	var overrides configOverrideValues
	fs.Var(&overrides, "set", "override a config value after loading config, repeatable; use dotted YAML paths like checks.quality=false")
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}
	if err := validateCommonFlags("", *profile); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitError
	}

	cfg, ok := loadConfigOrFail(*configPath, *profile, stderr)
	if !ok {
		return exitError
	}
	if overrideErr := applyConfigOverrides(&cfg, overrides); overrideErr != nil {
		_, _ = fmt.Fprintf(stderr, "invalid config override: %v\n", overrideErr)
		return exitError
	}
	if err := service.ValidateConfig(cfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "invalid config: %v\n", err)
		return exitError
	}
	if err := service.ValidateTargetPaths(cfg); err != nil {
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
	format := fs.String("format", "", "optional output format override: text, json, sarif, github, github-comment, cyclonedx")
	folderPath := fs.String("folder", "", "folder path to scan instead of all configured targets")
	pathAlias := fs.String("path", "", "alias for -folder")
	enableAI := fs.Bool("ai", false, "enable optional AI-assisted analysis")
	includeSuppressed := fs.Bool("include-suppressed", false, "include individual suppressed findings in JSON output")
	heapProfile := fs.String("memprofile", "", "optional path for rolling Go heap profiles during the scan")
	interactive := fs.Bool("interactive", false, "prompt for scan inputs in the terminal")
	outputFlags := registerScanOutputFlags(fs)
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}
	// Cheap flag checks run before any prompt, config load or progress line, so
	// a typo like -format sarf fails without printing "scan started".
	if err := validateCommonFlags(*format, *flags.profile); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitError
	}
	outputOverrides, err := outputFlags.configOverrides()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitError
	}
	configFlagSet := flagWasSet(fs, "config")
	flags.applyTrustPolicy()

	if err = promptScanInputs(*interactive, stdin, stdout, &inputs); err != nil {
		_, _ = fmt.Fprintf(stderr, "interactive scan: %v\n", err)
		return exitError
	}

	scanMode, err := parseScanMode(*inputs.mode)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitError
	}
	targetPath, err := scanTargetPath(*folderPath, *pathAlias)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitError
	}

	defaultConfigRequested := !configFlagSet && strings.TrimSpace(*inputs.configPath) == service.DefaultConfigPath()
	cfg, usedDefaults, err := loadScanConfigWithFallback(*inputs.configPath, *flags.profile, targetPath, defaultConfigRequested)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return exitError
	}
	if trimmedFormat := strings.TrimSpace(*format); trimmedFormat != "" {
		cfg.Output.Format = trimmedFormat
	}
	// Dedicated flags (-fail-on, -min-confidence) apply after -set, so they win.
	if err := applyConfigOverrides(&cfg, append(*flags.overrides, outputOverrides...)); err != nil {
		_, _ = fmt.Fprintf(stderr, "invalid config override: %v\n", err)
		return exitError
	}
	if usedDefaults {
		_, _ = fmt.Fprintln(stderr, configlessScanHint)
	}

	if err := executeScan(stdout, stderr, scanRequest{
		cfg: cfg, mode: scanMode, baseRef: strings.TrimSpace(*inputs.baseRef), targetPath: targetPath,
		enableAI: *enableAI, includeSuppressed: *includeSuppressed, heapProfilePath: *heapProfile, quiet: *outputFlags.quiet,
	}); err != nil {
		_, _ = fmt.Fprintf(stderr, "scan failed: %v\n", err)
		return exitError
	}
	return exitOK
}

func runValidatePatch(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate-patch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config file or directory path")
	format := fs.String("format", "", "optional output format override: text, json, sarif, github, github-comment, cyclonedx")
	enableAI := fs.Bool("ai", false, "enable optional AI-assisted analysis")
	profile := fs.String("profile", "", "optional policy profile override")
	var overrides configOverrideValues
	fs.Var(&overrides, "set", "override a config value after loading config, repeatable; use dotted YAML paths like checks.quality=false")
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}
	if err := validateCommonFlags(*format, *profile); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitError
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
	if overrideErr := applyConfigOverrides(&cfg, overrides); overrideErr != nil {
		_, _ = fmt.Fprintf(stderr, "invalid config override: %v\n", overrideErr)
		return exitError
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
	if err := service.ReportGateError(report, cfg.Output.FailOn); err != nil {
		_, _ = fmt.Fprintf(stderr, "patch validation failed: %v\n", err)
		return exitError
	}
	return exitOK
}

func runBaselineCreate(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("baseline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := registerScanRunFlags(fs)
	outputPath := fs.String("output", "codeguard-baseline.json", "baseline output path")
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}
	if err := validateCommonFlags("", *flags.profile); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitError
	}
	flags.applyTrustPolicy()

	cfg, ok := loadConfigOrFail(*flags.configPath, *flags.profile, stderr)
	if !ok {
		return exitError
	}
	cfg.Baseline.Path = ""
	if err := applyConfigOverrides(&cfg, *flags.overrides); err != nil {
		_, _ = fmt.Fprintf(stderr, "invalid config override: %v\n", err)
		return exitError
	}

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

// validateCommonFlags checks the -format and -profile flag values against the
// known formats and profiles without touching the filesystem. Empty values
// mean "use the config" and pass.
func validateCommonFlags(format string, profile string) error {
	if err := service.ValidateOutputFormat(format); err != nil {
		return fmt.Errorf("invalid -format: %w", err)
	}
	if err := service.ValidateProfileName(profile); err != nil {
		return fmt.Errorf("invalid -profile: %w", err)
	}
	return nil
}
