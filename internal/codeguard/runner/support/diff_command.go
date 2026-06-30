package support

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type diffCommandEnv struct {
	baseDir string
	headDir string
}

func prepareDiffCommandEnv(dir string, baseRef string) (diffCommandEnv, func(), error) {
	if err := ValidateBaseRef(baseRef); err != nil {
		return diffCommandEnv{}, func() {}, err
	}

	repoRoot, err := gitRepoRoot(dir)
	if err != nil {
		return diffCommandEnv{}, func() {}, err
	}

	repoRoot, err = canonicalPath(repoRoot)
	if err != nil {
		return diffCommandEnv{}, func() {}, fmt.Errorf("canonicalize repo root: %w", err)
	}
	dir, err = canonicalPath(dir)
	if err != nil {
		return diffCommandEnv{}, func() {}, fmt.Errorf("canonicalize target path: %w", err)
	}

	relativeTarget, err := filepath.Rel(repoRoot, dir)
	if err != nil {
		return diffCommandEnv{}, func() {}, fmt.Errorf("resolve target path: %w", err)
	}

	tempRoot, err := os.MkdirTemp("", "codeguard-diff-check-*")
	if err != nil {
		return diffCommandEnv{}, func() {}, err
	}

	headRoot := filepath.Join(tempRoot, "head")
	baseWorktree := filepath.Join(tempRoot, "base-worktree")
	cleanup := func() {
		// TODO(harden): thread caller ctx once prepareDiffCommandEnv accepts one.
		ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
		defer cancel()
		_ = exec.CommandContext(ctx, "git", "-C", repoRoot, "worktree", "remove", "--force", baseWorktree).Run() //nolint:gosec // fixed git subcommand; paths are tool-generated temp dirs
		_ = os.RemoveAll(tempRoot)
	}

	if err := copyDir(dir, headRoot); err != nil {
		cleanup()
		return diffCommandEnv{}, func() {}, fmt.Errorf("copy head target: %w", err)
	}

	// TODO(harden): thread caller ctx once prepareDiffCommandEnv accepts one.
	addCtx, addCancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer addCancel()
	cmd := exec.CommandContext(addCtx, "git", "-C", repoRoot, "worktree", "add", "--detach", "--end-of-options", baseWorktree, baseRef) //nolint:gosec // baseRef validated by ValidateBaseRef at function entry; --end-of-options blocks flag injection
	if output, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return diffCommandEnv{}, func() {}, fmt.Errorf("prepare base worktree for %q: %w: %s", baseRef, err, strings.TrimSpace(string(output)))
	}

	baseDir := filepath.Join(baseWorktree, relativeTarget)
	if info, err := os.Stat(baseDir); err != nil || !info.IsDir() {
		if err := os.MkdirAll(baseDir, 0o750); err != nil {
			cleanup()
			return diffCommandEnv{}, func() {}, fmt.Errorf("prepare base target dir: %w", err)
		}
	}

	return diffCommandEnv{
		baseDir: baseDir,
		headDir: headRoot,
	}, cleanup, nil
}

func canonicalPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if os.IsNotExist(err) {
		return path, nil
	}
	return "", err
}

func gitRepoRoot(dir string) (string, error) {
	// TODO(harden): thread caller ctx once gitRepoRoot accepts one.
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel") //nolint:gosec // fixed git subcommand; dir is a config scan target path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve git repo root for %q: %w: %s", dir, err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func copyDir(srcDir string, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == ".git" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dstPath := filepath.Join(dstDir, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(path, dstPath, info.Mode().Perm())
	})
}

func copyFile(srcPath string, dstPath string, mode os.FileMode) (err error) {
	src, err := os.Open(srcPath) //nolint:gosec // tool-generated source path during diff worktree copy
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := src.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	if err = os.MkdirAll(filepath.Dir(dstPath), 0o750); err != nil {
		return err
	}

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode) //nolint:gosec // tool-generated destination path during diff worktree copy
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := dst.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	_, err = io.Copy(dst, src)
	return err
}
