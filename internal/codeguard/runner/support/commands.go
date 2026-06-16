package support

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func RunCommandCheck(ctx context.Context, dir string, check core.CommandCheckConfig) (string, error) {
	return runCommandCheck(ctx, dir, check, nil)
}

func RunDiffCommandCheck(ctx context.Context, dir string, baseRef string, check core.CommandCheckConfig) (string, error) {
	diffEnv, cleanup, err := prepareDiffCommandEnv(dir, baseRef)
	if err != nil {
		return "", err
	}
	defer cleanup()

	return runDiffCommandCheck(ctx, diffEnv, baseRef, check)
}

func RunDiffCommandCheckWithContext(ctx context.Context, sc Context, dir string, baseRef string, check core.CommandCheckConfig) (string, error) {
	if diffEnv, ok := sc.DiffCommand[dir]; ok {
		return runDiffCommandCheck(ctx, diffEnv, baseRef, check)
	}
	return RunDiffCommandCheck(ctx, dir, baseRef, check)
}

func runDiffCommandCheck(ctx context.Context, diffEnv diffCommandEnv, baseRef string, check core.CommandCheckConfig) (string, error) {
	env := os.Environ()
	env = append(env,
		"CODEGUARD_DIFF_BASE_DIR="+diffEnv.baseDir,
		"CODEGUARD_DIFF_HEAD_DIR="+diffEnv.headDir,
		"CODEGUARD_DIFF_TARGET_DIR="+diffEnv.headDir,
		"CODEGUARD_DIFF_BASE_REF="+baseRef,
	)
	return runCommandCheck(ctx, diffEnv.headDir, check, env)
}

func runCommandCheck(ctx context.Context, dir string, check core.CommandCheckConfig, env []string) (string, error) {
	command := check.Command
	if strings.Contains(command, string(filepath.Separator)) && !filepath.IsAbs(command) {
		command = filepath.Join(dir, command)
	}
	cmd := exec.CommandContext(ctx, command, check.Args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = env
	}
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
