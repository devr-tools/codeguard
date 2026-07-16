// Package buildregression runs repository-configured build commands for the
// performance section's build-regression gate and provides bounded subprocess
// execution for wall-clock measurements.
package buildregression

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
	"github.com/devr-tools/codeguard/internal/codeguard/trust"
)

const (
	maxOutputBytes = 16 << 20
	runTimeout     = 15 * time.Minute
)

// Result is one measured build command duration.
type Result struct {
	Name           string
	DurationMillis float64
	Command        string
	TargetName     string
	TargetPath     string
}

// RunCommand executes one config-supplied build command in dir with a bounded
// output buffer and a contained timeout, returning the wall-clock duration in
// milliseconds plus the combined stdout+stderr text.
func RunCommand(ctx context.Context, dir string, target core.TargetConfig, check core.CommandCheckConfig) (Result, string, error) {
	if err := trust.GuardConfigCommand(check.Name, check.Command); err != nil {
		return Result{}, "", err
	}
	command := check.Command
	if strings.Contains(command, string(filepath.Separator)) && !filepath.IsAbs(command) {
		command = filepath.Join(dir, command)
	}
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, check.Args...) //nolint:gosec // command gated by trust.GuardConfigCommand above
	cmd.Dir = dir
	var buf bytes.Buffer
	limited := runnersupport.NewLimitedBufferWriter(&buf, maxOutputBytes)
	cmd.Stdout = limited
	cmd.Stderr = limited
	start := time.Now()
	err := cmd.Run()
	durationMillis := float64(time.Since(start)) / float64(time.Millisecond)
	if limited.Truncated() {
		return Result{}, "", fmt.Errorf("build command %q output exceeded %d bytes", check.Name, maxOutputBytes)
	}
	output := buf.String()
	result := Result{
		Name:           baselineKey(target, check),
		DurationMillis: durationMillis,
		Command:        check.Name,
		TargetName:     target.Name,
		TargetPath:     target.Path,
	}
	if err != nil {
		return result, output, fmt.Errorf("build command %q failed: %w", check.Name, err)
	}
	return result, output, nil
}

func baselineKey(target core.TargetConfig, check core.CommandCheckConfig) string {
	targetName := strings.TrimSpace(target.Name)
	if targetName == "" {
		targetName = strings.TrimSpace(target.Path)
	}
	return targetName + ":" + check.Name
}
