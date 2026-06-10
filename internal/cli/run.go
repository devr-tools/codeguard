package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	service "github.com/devr-tools/codeguard/codeguard"
	"github.com/devr-tools/codeguard/internal/version"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
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
		return runInit(args[1:], stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "scan":
		return runScan(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		writeUsage(stderr)
		return 1
	}
}

func runInit(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", "codeguard.json", "output config path")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if err := service.WriteConfigFile(*output, service.ExampleConfig()); err != nil {
		_, _ = fmt.Fprintf(stderr, "write config: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "wrote %s\n", *output)
	return 0
}

func runValidate(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "codeguard.json", "config path")
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

func runScan(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "config path")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg := service.ExampleConfig()
	if strings.TrimSpace(*configPath) != "" {
		loaded, err := service.LoadConfigFile(*configPath)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
			return 1
		}
		cfg = loaded
	}

	report, err := service.NewRunner(cfg).Run(context.Background())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "scan failed: %v\n", err)
		return 1
	}
	if err := service.WriteReport(stdout, report, cfg.Output.Format); err != nil {
		_, _ = fmt.Fprintf(stderr, "write report: %v\n", err)
		return 1
	}

	if report.Summary.FailedSections > 0 {
		return 1
	}
	return 0
}

func writeUsage(w io.Writer) {
	_, _ = io.WriteString(w, `codeguard

Usage:
  codeguard init [-output codeguard.json]
  codeguard validate [-config codeguard.json]
  codeguard scan [-config codeguard.json]
  codeguard version
`)
}
