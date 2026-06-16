package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const commandEnvKey = "CODEGUARD_SEMANTIC_COMMAND"

func commandConfigured(command string) bool {
	return strings.TrimSpace(command) != ""
}

func runCommand(ctx context.Context, command string, req Request) (Response, error) {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return Response{}, fmt.Errorf("semantic command is not configured")
	}
	input, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stdin = bytes.NewReader(input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Response{}, fmt.Errorf("semantic command failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return Response{}, fmt.Errorf("semantic command returned invalid JSON: %w", err)
	}
	return resp, nil
}
