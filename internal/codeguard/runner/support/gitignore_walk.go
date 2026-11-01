package support

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/execlimit"
)

// maxGitignoreFileBytes bounds one ignore file. The repository under scan may
// be an untrusted pull request, and every walked entry is matched against
// every applicable pattern, so an enormous ignore file would turn the walk into
// a denial of service. An ignore file over the cap is not honoured at all:
// failing toward scanning more is the safe direction for a scanner, whereas
// honouring a truncated prefix would silently change what a later negation
// means.
const maxGitignoreFileBytes = 256 << 10

// gitTrackedListTimeout and maxGitTrackedListBytes bound the one
// `git ls-files` run per repository that finds tracked-but-ignored files.
const gitTrackedListTimeout = 20 * time.Second
const maxGitTrackedListBytes = 64 << 20

// gitignoreRules is the pattern list of one ignore file, with base the
// repository-relative directory its patterns are relative to ("" is the
// repository root).
type gitignoreRules struct {
	base     string
	patterns []gitignorePattern
}

// match returns (ignored, true) for the last pattern matching repoRel, or
// (false, false) when no pattern in this file decides it.
func (r *gitignoreRules) match(repoRel string, isDir bool) (bool, bool) {
	rel := repoRel
	if r.base != "" {
		if !strings.HasPrefix(repoRel, r.base+"/") {
			return false, false
		}
		rel = repoRel[len(r.base)+1:]
	}
	for i := len(r.patterns) - 1; i >= 0; i-- {
		if r.patterns[i].matches(rel, isDir) {
			return !r.patterns[i].negate, true
		}
	}
	return false, false
}

// gitignoreRepo is one git working tree met by the walk. Nested repositories
// (submodules, vendored checkouts, worktrees) start their own rule set: git
// never applies a parent repository's ignore files inside a nested one.
type gitignoreRepo struct {
	root    string
	exclude *gitignoreRules
	// tracked lists files that are both ignored and tracked, as repository-
	// relative paths, and trackedDirs every ancestor directory of one. Git
	// keeps tracking a file that was committed before (or force-added
	// despite) an ignore rule, and so does the scan.
	tracked       map[string]struct{}
	trackedDirs   map[string]struct{}
	trackedLoaded bool
	// disabled is set when the tracked set could not be listed. Without it the
	// walk cannot tell committed code from ignored output, and dropping
	// committed code is the worse failure for a scanner, so the repository is
	// then walked as if it had no ignore rules.
	disabled bool
}

// gitignoreDir is the ignore state of one walked directory.
type gitignoreDir struct {
	parent  *gitignoreDir
	repo    *gitignoreRepo
	repoRel string
	rules   *gitignoreRules
	// ignored marks a directory that is itself ignored but was entered anyway,
	// because it holds tracked files or a codeguard re-include may match inside
	// it. Its untracked contents remain ignored.
	ignored bool
}

// gitignoreWalker applies .gitignore semantics to one filepath.WalkDir pass.
// It is driven by a single goroutine (the walk callback) and needs no locking.
type gitignoreWalker struct {
	dirs map[string]*gitignoreDir
	// listTracked returns the repository-relative paths of tracked files
	// that git considers ignored.
	listTracked func(repoRoot string) ([]string, error)
}

// newGitignoreWalker prepares ignore matching for a walk rooted at root. It
// returns nil when ignore rules cannot apply: root is not inside a git working
// tree, or root itself lies inside an ignored directory. The second case is
// the explicit-target rule: a target or -folder the user pointed at is always
// scanned, even when the repository ignores it.
func newGitignoreWalker(root string) *gitignoreWalker {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	walker := &gitignoreWalker{dirs: map[string]*gitignoreDir{}, listTracked: gitListTrackedIgnored}
	repoRoot, ok := findGitWorkTree(abs)
	if !ok {
		// Not inside a repository, but nested repositories below root still
		// carry their own rules.
		walker.dirs["."] = walker.enterDir(nil, abs, "", false)
		return walker
	}
	rel, err := filepath.Rel(repoRoot, abs)
	if err != nil {
		return nil
	}
	repo := newGitignoreRepo(repoRoot)
	node := &gitignoreDir{repo: repo, rules: loadGitignoreRules(repoRoot, "")}
	if rel = filepath.ToSlash(rel); rel != "." {
		for _, segment := range strings.Split(rel, "/") {
			repoRel := path.Join(node.repoRel, segment)
			if repo.matches(node, repoRel, true) {
				return nil
			}
			node = &gitignoreDir{
				parent:  node,
				repo:    repo,
				repoRel: repoRel,
				rules:   loadGitignoreRules(filepath.Join(repoRoot, filepath.FromSlash(repoRel)), repoRel),
			}
		}
	}
	walker.dirs["."] = node
	return walker
}

// skip reports whether the entry at walk-relative rel is ignored. For a
// directory that is not skipped it also records the directory's own ignore
// state for its children; keepDir says whether an ignored directory must still
// be entered (a codeguard re-include may match inside it).
func (w *gitignoreWalker) skip(absPath string, rel string, isDir bool, reincluded bool, keepDir bool) bool {
	parent := w.dirs[path.Dir(rel)]
	if parent == nil {
		return false
	}
	name := path.Base(rel)
	ignored, tracked := w.decide(parent, name, isDir)
	if !isDir {
		return ignored && !tracked && !reincluded
	}
	if ignored && !tracked && !reincluded && !keepDir {
		return true
	}
	repoRel := ""
	if parent.repo != nil {
		repoRel = path.Join(parent.repoRel, name)
	}
	w.dirs[rel] = w.enterDir(parent, absPath, repoRel, ignored && !reincluded)
	return false
}

// decide returns whether name inside dir is ignored and, if so, whether it is
// tracked (a tracked file, or a directory holding one).
func (w *gitignoreWalker) decide(dir *gitignoreDir, name string, isDir bool) (bool, bool) {
	repo := dir.repo
	if repo == nil || repo.disabled {
		return false, false
	}
	repoRel := path.Join(dir.repoRel, name)
	ignored := dir.ignored || repo.matches(dir, repoRel, isDir)
	if !ignored {
		return false, false
	}
	w.loadTracked(repo)
	if repo.disabled {
		return false, false
	}
	if isDir {
		_, tracked := repo.trackedDirs[repoRel]
		return true, tracked
	}
	_, tracked := repo.tracked[repoRel]
	return true, tracked
}

// matches evaluates the ignore files from the most specific (the closest
// .gitignore, last line first) to the least (info/exclude); the first file
// with a matching pattern decides.
func (r *gitignoreRepo) matches(dir *gitignoreDir, repoRel string, isDir bool) bool {
	for d := dir; d != nil && d.repo == r; d = d.parent {
		if d.rules == nil {
			continue
		}
		if ignored, ok := d.rules.match(repoRel, isDir); ok {
			return ignored
		}
	}
	if r.exclude != nil {
		if ignored, ok := r.exclude.match(repoRel, isDir); ok {
			return ignored
		}
	}
	return false
}

// enterDir builds the state for a directory the walk is about to descend
// into, starting a fresh repository when the directory is a nested work tree.
func (w *gitignoreWalker) enterDir(parent *gitignoreDir, absPath string, repoRel string, ignored bool) *gitignoreDir {
	if parent != nil && isGitWorkTreeRoot(absPath) {
		repo := newGitignoreRepo(absPath)
		return &gitignoreDir{parent: parent, repo: repo, rules: loadGitignoreRules(absPath, "")}
	}
	node := &gitignoreDir{parent: parent, repoRel: repoRel, ignored: ignored}
	if parent != nil {
		node.repo = parent.repo
	}
	if node.repo != nil && !ignored {
		node.rules = loadGitignoreRules(absPath, repoRel)
	}
	return node
}

func (w *gitignoreWalker) loadTracked(repo *gitignoreRepo) {
	if repo.trackedLoaded {
		return
	}
	repo.trackedLoaded = true
	paths, err := w.listTracked(repo.root)
	if err != nil {
		repo.disabled = true
		return
	}
	repo.tracked = make(map[string]struct{}, len(paths))
	repo.trackedDirs = map[string]struct{}{}
	for _, p := range paths {
		repo.tracked[p] = struct{}{}
		for dir := path.Dir(p); dir != "." && dir != "/"; dir = path.Dir(dir) {
			if _, seen := repo.trackedDirs[dir]; seen {
				break
			}
			repo.trackedDirs[dir] = struct{}{}
		}
	}
}

func newGitignoreRepo(root string) *gitignoreRepo {
	repo := &gitignoreRepo{root: root}
	if gitDir, ok := resolveGitDir(root); ok {
		if data, ok := readGitignoreFile(filepath.Join(gitCommonDir(gitDir), "info", "exclude")); ok {
			repo.exclude = &gitignoreRules{patterns: parseGitignore(data)}
		}
	}
	return repo
}

func loadGitignoreRules(dir string, base string) *gitignoreRules {
	data, ok := readGitignoreFile(filepath.Join(dir, ".gitignore"))
	if !ok {
		return nil
	}
	patterns := parseGitignore(data)
	if len(patterns) == 0 {
		return nil
	}
	return &gitignoreRules{base: base, patterns: patterns}
}

// readGitignoreFile reads an ignore file if it is a regular file (git does not
// follow a symlinked .gitignore) within maxGitignoreFileBytes.
func readGitignoreFile(path string) ([]byte, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxGitignoreFileBytes {
		return nil, false
	}
	f, err := os.Open(path) //nolint:gosec // fixed ignore-file name inside the scanned tree or its git dir
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxGitignoreFileBytes+1))
	if err != nil || len(data) > maxGitignoreFileBytes {
		return nil, false
	}
	return data, true
}

// findGitWorkTree returns the nearest ancestor of dir (inclusive) that is a
// git working tree root.
func findGitWorkTree(dir string) (string, bool) {
	for {
		if isGitWorkTreeRoot(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// isGitWorkTreeRoot reports whether dir holds a .git directory or a .git file
// (worktrees and submodules use a "gitdir:" file).
func isGitWorkTreeRoot(dir string) bool {
	info, err := os.Lstat(filepath.Join(dir, ".git"))
	return err == nil && (info.IsDir() || info.Mode().IsRegular())
}

// resolveGitDir returns the git directory of the working tree at root,
// following a "gitdir:" file.
func resolveGitDir(root string) (string, bool) {
	dotGit := filepath.Join(root, ".git")
	info, err := os.Lstat(dotGit)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return dotGit, true
	}
	data, ok := readGitignoreFile(dotGit)
	if !ok {
		return "", false
	}
	line := strings.TrimSpace(string(data))
	gitDir, found := strings.CutPrefix(line, "gitdir:")
	if !found {
		return "", false
	}
	gitDir = strings.TrimSpace(gitDir)
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	return filepath.Clean(gitDir), true
}

// gitCommonDir returns the directory shared by all worktrees of a repository,
// which is where info/exclude lives.
func gitCommonDir(gitDir string) string {
	data, ok := readGitignoreFile(filepath.Join(gitDir, "commondir"))
	if !ok {
		return gitDir
	}
	common := strings.TrimSpace(string(data))
	if common == "" {
		return gitDir
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(gitDir, common)
	}
	return filepath.Clean(common)
}

// gitListTrackedIgnored lists, relative to repoRoot, the tracked files that
// match an ignore rule. Only those can differ between "ignored" and "scanned",
// so the full index is never loaded into memory. core.fsmonitor is forced off
// so the repository's own config cannot run a hook from the scan.
func gitListTrackedIgnored(repoRoot string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTrackedListTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-c", "core.fsmonitor=false", "-C", repoRoot, "ls-files", "-z", "--cached", "--ignored", "--exclude-standard") //nolint:gosec // fixed git subcommand; repoRoot is a scan target ancestor
	var out bytes.Buffer
	limited := execlimit.NewWriter(&out, maxGitTrackedListBytes)
	cmd.Stdout = limited
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	if limited.Truncated() {
		return nil, io.ErrShortBuffer
	}
	var paths []string
	for _, entry := range bytes.Split(out.Bytes(), []byte{0}) {
		if len(entry) > 0 {
			paths = append(paths, string(entry))
		}
	}
	return paths, nil
}
