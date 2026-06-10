package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	service "github.com/devr-tools/codeguard/codeguard"
	"github.com/devr-tools/codeguard/internal/version"
)

func Run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		writeUsage(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		writeUsage(stdout)
		return 0
	case "version":
		_, _ = fmt.Fprintln(stdout, version.Number)
		return 0
	case "init":
		return runInit(args[1:], stdin, stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "scan":
		return runScan(args[1:], stdin, stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		writeUsage(stderr)
		return 1
	}
}

func runInit(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", service.DefaultConfigPath(), "output config path")
	interactive := fs.Bool("interactive", false, "prompt for config values in the terminal")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg := service.ExampleConfig()
	if *interactive {
		reader := bufio.NewReader(stdin)
		var err error
		*output, err = promptString(reader, stdout, "config output path", *output)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "interactive init: %v\n", err)
			return 1
		}
		cfg.Name, err = promptString(reader, stdout, "config name", cfg.Name)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "interactive init: %v\n", err)
			return 1
		}
	}

	if err := service.WriteConfigFile(*output, cfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "write config: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "wrote %s\n", *output)
	return 0
}

func runValidate(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config file or directory path")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg, err := service.LoadConfigFile(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	if err := service.ValidateConfig(cfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "invalid config: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintln(stdout, "config valid")
	return 0
}

func runScan(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	inputs := scanInputs{
		configPath: fs.String("config", service.DefaultConfigPath(), "config file or directory path"),
		mode:       fs.String("mode", string(service.ScanModeFull), "scan mode: full or diff"),
		baseRef:    fs.String("base-ref", "main", "base branch/ref for diff mode"),
	}
	interactive := fs.Bool("interactive", false, "prompt for scan inputs in the terminal")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if err := promptScanInputs(*interactive, stdin, stdout, &inputs); err != nil {
		_, _ = fmt.Fprintf(stderr, "interactive scan: %v\n", err)
		return 1
	}

	scanMode, err := parseScanMode(*inputs.mode)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	cfg, err := service.LoadConfigFile(*inputs.configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}

	if err := executeScan(stdout, cfg, scanMode, strings.TrimSpace(*inputs.baseRef)); err != nil {
		_, _ = fmt.Fprintf(stderr, "scan failed: %v\n", err)
		return 1
	}
	return 0
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

func executeScan(stdout io.Writer, cfg service.Config, scanMode service.ScanMode, baseRef string) error {
	report, err := service.RunWithOptions(context.Background(), cfg, service.ScanOptions{
		Mode:    scanMode,
		BaseRef: baseRef,
	})
	if err != nil {
		return err
	}
	if err := service.WriteReport(stdout, report, cfg.Output.Format); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	if report.Summary.FailedSections > 0 {
		return fmt.Errorf("one or more sections failed")
	}
	return nil
}

func promptString(reader *bufio.Reader, stdout io.Writer, label string, fallback string) (string, error) {
	if _, err := fmt.Fprintf(stdout, "%s [%s]: ", label, fallback); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return fallback, nil
	}
	return line, nil
}

func writeUsage(w io.Writer) {
	_, _ = io.WriteString(w, `codeguard

Usage:
  codeguard init [-output codeguard.yaml] [-interactive]
  codeguard validate [-config codeguard.yaml]
  codeguard scan [-config codeguard.yaml] [-mode full|diff] [-base-ref main] [-interactive]
  codeguard version
`)
}
