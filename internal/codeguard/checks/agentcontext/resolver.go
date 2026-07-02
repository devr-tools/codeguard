package agentcontext

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// maxDocFileBytes caps how much of a documentation or build file this section
// reads: agent docs, READMEs, Makefiles, and package.json files are always far
// smaller, and the cap keeps a pathological file from ballooning scan memory.
const maxDocFileBytes = 4 << 20

// repoResolver answers "does this doc reference resolve?" questions against a
// single target root. Every resolution is deliberately conservative: when the
// resolver cannot positively prove a reference broken (no Makefile, a Makefile
// with includes or pattern rules, a workspace-style package.json), it reports
// the reference as fine. False positives kill drift rules; precision wins.
type repoResolver struct {
	root         string
	makeTargets  map[string]struct{}
	makeReliable bool
	npmScripts   map[string]struct{}
	npmReliable  bool
	pathCache    map[string]bool
}

func newRepoResolver(root string) *repoResolver {
	r := &repoResolver{root: root, pathCache: map[string]bool{}}
	r.loadMakeTargets()
	r.loadNpmScripts()
	return r
}

// pathExists reports whether the repo-relative path exists under the root.
func (r *repoResolver) pathExists(rel string) bool {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	rel = strings.TrimSuffix(rel, "/")
	if rel == "" || rel == "." {
		return true
	}
	if cached, ok := r.pathCache[rel]; ok {
		return cached
	}
	_, err := os.Stat(filepath.Join(r.root, filepath.FromSlash(rel)))
	exists := err == nil
	r.pathCache[rel] = exists
	return exists
}

// makeTargetMissing reports whether name is provably absent from the root
// Makefile. It returns false whenever the Makefile could define targets the
// parser cannot see (includes, pattern rules) or does not exist at all.
func (r *repoResolver) makeTargetMissing(name string) bool {
	if !r.makeReliable {
		return false
	}
	_, ok := r.makeTargets[name]
	return !ok
}

// npmScriptMissing reports whether the script is provably absent from the
// root package.json scripts map.
func (r *repoResolver) npmScriptMissing(name string) bool {
	if !r.npmReliable {
		return false
	}
	_, ok := r.npmScripts[name]
	return !ok
}

func (r *repoResolver) loadMakeTargets() {
	data, ok := readCappedDocFile(r.root, "Makefile", "makefile", "GNUmakefile")
	if !ok {
		return
	}
	r.makeTargets = map[string]struct{}{}
	r.makeReliable = true
	for _, line := range strings.Split(string(data), "\n") {
		if !r.recordMakeLine(line) {
			r.makeReliable = false
			return
		}
	}
}

// recordMakeLine parses one Makefile line into the target set. It returns
// false when the line makes static target resolution unreliable (an include
// directive or a pattern rule), which disables make-target drift checks.
func (r *repoResolver) recordMakeLine(line string) bool {
	if strings.HasPrefix(line, "\t") {
		return true
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return true
	}
	if isMakeIncludeDirective(trimmed) {
		return false
	}
	idx := strings.IndexByte(trimmed, ':')
	if idx <= 0 || strings.HasPrefix(trimmed[idx:], ":=") || strings.ContainsAny(trimmed[:idx], "=$") {
		return true
	}
	names, right := strings.Fields(trimmed[:idx]), trimmed[idx+1:]
	if strings.Contains(trimmed[:idx], "%") {
		return false
	}
	// Special targets such as .PHONY declare their real targets on the right.
	if len(names) == 1 && strings.HasPrefix(names[0], ".") && strings.ToUpper(names[0]) == names[0] {
		names = strings.Fields(strings.TrimPrefix(right, ":"))
	}
	for _, name := range names {
		if !strings.ContainsAny(name, "$%") {
			r.makeTargets[strings.TrimSuffix(name, ":")] = struct{}{}
		}
	}
	return true
}

func isMakeIncludeDirective(trimmed string) bool {
	for _, directive := range []string{"include ", "-include ", "sinclude "} {
		if strings.HasPrefix(trimmed, directive) {
			return true
		}
	}
	return false
}

func (r *repoResolver) loadNpmScripts() {
	data, ok := readCappedDocFile(r.root, "package.json")
	if !ok {
		return
	}
	var manifest struct {
		Scripts    map[string]string `json:"scripts"`
		Workspaces json.RawMessage   `json:"workspaces"`
	}
	// Workspace roots delegate scripts to member packages, so absence at the
	// root proves nothing; leave the resolver unreliable in that case.
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Workspaces != nil {
		return
	}
	r.npmScripts = map[string]struct{}{}
	r.npmReliable = true
	for name := range manifest.Scripts {
		r.npmScripts[name] = struct{}{}
	}
}

// readCappedDocFile returns the first existing candidate file under root,
// skipping any candidate larger than maxDocFileBytes.
func readCappedDocFile(root string, candidates ...string) ([]byte, bool) {
	for _, name := range candidates {
		path := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() > maxDocFileBytes {
			continue
		}
		data, err := os.ReadFile(path) //nolint:gosec // fixed doc/build file names joined under the scan root
		if err != nil {
			continue
		}
		return data, true
	}
	return nil, false
}
