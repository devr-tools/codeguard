package nlrule

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type disabledRuntime struct{}

func (disabledRuntime) Enabled() bool { return false }

func (disabledRuntime) Fingerprint() string { return "disabled" }

func (disabledRuntime) Evaluate(context.Context, EvaluationRequest) (EvaluationResponse, error) {
	return EvaluationResponse{}, nil
}

type commandRuntime struct {
	command string
}

func NewRuntime(cfg core.AIConfig) Runtime {
	command := runtimeCommand(cfg)
	if command == "" {
		return disabledRuntime{}
	}
	return commandRuntime{command: command}
}

func NewRuntimeFromEnv() Runtime {
	return NewRuntime(core.AIConfig{})
}

func (runtime commandRuntime) Enabled() bool { return true }

func (runtime commandRuntime) Fingerprint() string {
	info, err := os.Stat(runtime.command)
	if err != nil {
		return "command:" + runtime.command
	}
	return strings.Join([]string{
		"command",
		runtime.command,
		strconv.FormatInt(info.Size(), 10),
		strconv.FormatInt(info.ModTime().Unix(), 10),
	}, ":")
}

func (runtime commandRuntime) Evaluate(ctx context.Context, request EvaluationRequest) (EvaluationResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return EvaluationResponse{}, err
	}
	cmd := exec.CommandContext(ctx, runtime.command)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return EvaluationResponse{}, fmt.Errorf("nlrule runtime %s: %w%s", runtime.command, err, formatStderr(stderr.String()))
	}
	var response EvaluationResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return EvaluationResponse{}, fmt.Errorf("nlrule runtime %s returned invalid json: %w", runtime.command, err)
	}
	return response, nil
}
