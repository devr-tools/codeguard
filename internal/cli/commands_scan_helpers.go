package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

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
