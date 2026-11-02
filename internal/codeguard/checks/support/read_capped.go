package support

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// MaxTargetFileBytes caps how much of one repository file codeguard reads into
// memory. The runner's walk skips larger files and its corpus refuses to buffer
// them; direct reads in check packages use the same cap through
// ReadCappedFile, because the repository may be an untrusted pull request and
// one multi-gigabyte file must not be able to exhaust the CI runner.
const MaxTargetFileBytes = 32 << 20 // 32 MiB

// ErrFileTooLarge is wrapped by the error ReadCappedFile returns for a file
// larger than its cap.
var ErrFileTooLarge = fmt.Errorf("file exceeds the %d byte scan limit", MaxTargetFileBytes)

// ReadCappedFile reads path without ever buffering more than
// MaxTargetFileBytes. It reads one byte past the cap to tell an exactly
// cap-sized file from an oversized one, and fails with ErrFileTooLarge for the
// latter rather than returning a truncated prefix.
func ReadCappedFile(path string) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // callers pass paths under the configured scan target
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, MaxTargetFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxTargetFileBytes {
		return nil, fmt.Errorf("%s: %w", path, ErrFileTooLarge)
	}
	return data, nil
}

// ReadTargetFileCapped returns the bytes of target-root-relative rel through
// the shared corpus (env.ReadTargetFile) when the context has one, and through
// a capped direct read otherwise. It never falls back to an uncapped read: a
// corpus error (including an oversized file) is returned as is.
func ReadTargetFileCapped(env Context, target core.TargetConfig, rel string) ([]byte, error) {
	if env.ReadTargetFile != nil {
		return env.ReadTargetFile(target, rel)
	}
	return ReadCappedFile(filepath.Join(target.Path, filepath.FromSlash(rel)))
}
