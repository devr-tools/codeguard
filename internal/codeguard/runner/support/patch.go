package support

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func LoadDiffScopeFromUnifiedDiff(targets []core.TargetConfig, diffText string) map[string]LineRanges {
	out := map[string]LineRanges{}
	for _, target := range targets {
		scope := parseUnifiedDiff(RebaseUnifiedDiff(diffText, DiffPrefixForTarget(target.Path)))
		for path, ranges := range scope {
			out[path] = ranges
		}
	}
	return out
}

func MaterializePatchedTargets(cfg core.Config, diffText string) (core.Config, map[string]diffCommandEnv, func(), error) {
	tempRoot, err := os.MkdirTemp("", "codeguard-patch-*")
	if err != nil {
		return core.Config{}, nil, func() {}, err
	}

	cleanup := func() {
		_ = os.RemoveAll(tempRoot)
	}

	patched := cfg
	patched.Targets = append([]core.TargetConfig(nil), cfg.Targets...)
	diffCommand := make(map[string]diffCommandEnv, len(cfg.Targets))
	for i, target := range cfg.Targets {
		targetRoot := filepath.Join(tempRoot, fmt.Sprintf("target-%d", i))
		baseDir := filepath.Join(targetRoot, "base")
		headDir := filepath.Join(targetRoot, "head")
		if err := copyDir(target.Path, baseDir); err != nil {
			cleanup()
			return core.Config{}, nil, func() {}, fmt.Errorf("copy base target %q: %w", target.Name, err)
		}
		if err := copyDir(target.Path, headDir); err != nil {
			cleanup()
			return core.Config{}, nil, func() {}, fmt.Errorf("copy head target %q: %w", target.Name, err)
		}

		targetDiff := strings.TrimSpace(RebaseUnifiedDiff(diffText, DiffPrefixForTarget(target.Path)))
		if targetDiff != "" {
			if err := applyUnifiedDiff(headDir, targetDiff+"\n"); err != nil {
				cleanup()
				return core.Config{}, nil, func() {}, fmt.Errorf("apply patch for target %q: %w", target.Name, err)
			}
		}

		patched.Targets[i].Path = headDir
		diffCommand[headDir] = diffCommandEnv{
			baseDir: baseDir,
			headDir: headDir,
		}
	}
	return patched, diffCommand, cleanup, nil
}

// ApplyUnifiedDiff applies diffText to each configured target in place, rebasing
// per target the same way MaterializePatchedTargets does for verification. It is
// used to write a verified fix to the working tree. Targets whose rebased diff
// is empty are skipped.
func ApplyUnifiedDiff(cfg core.Config, diffText string) error {
	for _, target := range cfg.Targets {
		targetDiff := strings.TrimSpace(RebaseUnifiedDiff(diffText, DiffPrefixForTarget(target.Path)))
		if targetDiff == "" {
			continue
		}
		if err := applyUnifiedDiff(target.Path, targetDiff+"\n"); err != nil {
			return fmt.Errorf("apply patch for target %q: %w", target.Name, err)
		}
	}
	return nil
}

func applyUnifiedDiff(dir string, diffText string) error {
	cmd := exec.Command("git", "apply", "--recount", "--whitespace=nowarn")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(diffText)
	output, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(output))
		if text == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, text)
	}
	return nil
}

// DiffPrefixForTarget resolves the repo-relative prefix of a target directory
// so unified diffs can be rebased onto target-relative paths.
func DiffPrefixForTarget(dir string) string {
	repoRoot, err := gitRepoRoot(dir)
	if err != nil {
		return ""
	}

	repoRoot, err = canonicalPath(repoRoot)
	if err != nil {
		return ""
	}
	dir, err = canonicalPath(dir)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(repoRoot, dir)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return ""
	}
	return strings.Trim(rel, "/")
}

func RebaseUnifiedDiff(diffText string, prefix string) string {
	blocks := splitUnifiedDiffBlocks(diffText)
	prefix = strings.Trim(strings.TrimSpace(filepath.ToSlash(prefix)), "/")
	if len(blocks) == 0 {
		return ""
	}

	var kept []string
	for _, block := range blocks {
		rewritten, ok := rebaseUnifiedDiffBlock(block, prefix)
		if ok {
			kept = append(kept, rewritten)
		}
	}
	return strings.Join(kept, "")
}

func splitUnifiedDiffBlocks(diffText string) []string {
	lines := strings.SplitAfter(diffText, "\n")
	if len(lines) == 0 {
		return nil
	}

	var blocks []string
	var current []string
	for _, line := range lines {
		if startsUnifiedDiffBlock(line) && len(current) > 0 {
			blocks = append(blocks, strings.Join(current, ""))
			current = current[:0]
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		blocks = append(blocks, strings.Join(current, ""))
	}
	return blocks
}

func startsUnifiedDiffBlock(line string) bool {
	return strings.HasPrefix(line, "diff --git ") || strings.HasPrefix(line, "--- ")
}
