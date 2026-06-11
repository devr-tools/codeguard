package support

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func RunCommandCheck(ctx context.Context, dir string, check core.CommandCheckConfig) (string, error) {
	command := check.Command
	if strings.Contains(command, string(filepath.Separator)) && !filepath.IsAbs(command) {
		command = filepath.Join(dir, command)
	}
	cmd := exec.CommandContext(ctx, command, check.Args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			return "", fmt.Errorf("%s failed: %w", check.Name, err)
		}
		return text, fmt.Errorf("%s failed: %w", check.Name, err)
	}
	return text, nil
}
