package support

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

func RunLimitedCommand(ctx context.Context, dir string, maxOutputBytes int, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // caller validates untrusted args before invoking this bounded subprocess helper
	cmd.Dir = dir
	var buf bytes.Buffer
	limited := NewLimitedBufferWriter(&buf, maxOutputBytes)
	cmd.Stdout = limited
	cmd.Stderr = limited
	err := cmd.Run()
	if limited.Truncated() {
		return "", fmt.Errorf("%s output exceeded %d bytes", name, maxOutputBytes)
	}
	return buf.String(), err
}
