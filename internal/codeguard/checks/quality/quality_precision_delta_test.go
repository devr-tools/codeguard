package quality

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestReadCurrentTargetFileDoesNotBypassConfiguredReaderError(t *testing.T) {
	target := core.TargetConfig{Path: t.TempDir()}
	if err := os.WriteFile(filepath.Join(target.Path, "oversized.go"), []byte("package sample"), 0o600); err != nil {
		t.Fatalf("write current target file: %v", err)
	}
	env := support.Context{
		ReadTargetFile: func(core.TargetConfig, string) ([]byte, error) {
			return nil, errors.New("file exceeds scan limit")
		},
	}

	data, ok := readCurrentTargetFile(env, target, "oversized.go")
	if ok || data != nil {
		t.Fatalf("readCurrentTargetFile() = (%q, %t), want (nil, false)", data, ok)
	}
}
