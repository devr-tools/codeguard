package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// doctorLanguageWalkLimit caps how many directory entries the language probe
// visits, so doctor stays fast on a huge monorepo; hitting the cap counts as
// "not sure" and stays silent rather than warning.
const doctorLanguageWalkLimit = 200_000

// doctorLanguageExtensions maps a configured target language to the source
// extensions that language's checks read. Languages missing here (custom or
// future ones) are not probed.
var doctorLanguageExtensions = func() map[string][]string {
	goExts := []string{".go"}
	pythonExts := []string{".py"}
	scriptExts := []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"}
	cppExts := []string{".cc", ".cpp", ".cxx", ".c", ".h", ".hpp", ".hh", ".hxx"}
	return map[string][]string{
		"go": goExts, "golang": goExts,
		"python": pythonExts, "py": pythonExts,
		"typescript": scriptExts, "javascript": scriptExts, "ts": scriptExts, "tsx": scriptExts, "js": scriptExts, "jsx": scriptExts,
		"cpp": cppExts, "c++": cppExts, "cxx": cppExts, "c": cppExts,
		"rust": {".rs"}, "java": {".java"}, "csharp": {".cs"}, "ruby": {".rb"},
	}
}()

// targetLayoutDoctorChecks covers the config mistakes that make a scan
// silently scan nothing useful: entrypoints that do not exist (design
// reachability then anchors on nothing) and a target language with no matching
// source under the target path (every language check then passes vacuously).
func targetLayoutDoctorChecks(cfg service.Config) []doctorCheck {
	checks := make([]doctorCheck, 0, len(cfg.Targets))
	for _, target := range cfg.Targets {
		if !targetPathExists(target.Path) {
			continue // targetDoctorChecks already failed this target.
		}
		checks = append(checks, entrypointDoctorChecks(target)...)
		if check, ok := languageDoctorCheck(target, cfg.Exclude); ok {
			checks = append(checks, check)
		}
	}
	return checks
}

func entrypointDoctorChecks(target service.TargetConfig) []doctorCheck {
	if len(target.Entrypoints) == 0 {
		return nil
	}
	missing := make([]string, 0)
	for _, entrypoint := range target.Entrypoints {
		if !entrypointExists(target.Path, entrypoint) {
			missing = append(missing, entrypoint)
		}
	}
	name := "entrypoints:" + target.Name
	if len(missing) > 0 {
		return []doctorCheck{failDoctorCheck(name, fmt.Sprintf("%d of %d configured entrypoint(s) do not exist under %s: %s",
			len(missing), len(target.Entrypoints), target.Path, strings.Join(missing, ", ")))}
	}
	return []doctorCheck{passDoctorCheck(name, fmt.Sprintf("all %d configured entrypoint(s) exist", len(target.Entrypoints)))}
}

// entrypointExists resolves an entrypoint relative to its target path; a glob
// pattern counts as present when it matches at least one path.
func entrypointExists(targetPath string, entrypoint string) bool {
	entrypoint = strings.TrimSpace(entrypoint)
	if entrypoint == "" {
		return false
	}
	full := filepath.Join(targetPath, filepath.FromSlash(entrypoint))
	if wildcard := strings.IndexAny(entrypoint, "*?["); wildcard >= 0 {
		if matches, err := filepath.Glob(full); err == nil && len(matches) > 0 {
			return true
		}
		// filepath.Glob has no "**"; for those patterns only the literal
		// directory before the first wildcard can be checked.
		if strings.Contains(entrypoint, "**") {
			_, err := os.Stat(filepath.Join(targetPath, filepath.FromSlash(path.Dir(entrypoint[:wildcard]+"x"))))
			return err == nil
		}
		return false
	}
	_, err := os.Stat(full)
	return err == nil
}

func languageDoctorCheck(target service.TargetConfig, excludes []string) (doctorCheck, bool) {
	language := normalizedLanguage(target.Language)
	extensions, known := doctorLanguageExtensions[language]
	if language == "" || !known {
		return doctorCheck{}, false
	}
	found, others, complete := probeTargetLanguages(target.Path, extensions, excludes)
	if found || !complete {
		return doctorCheck{}, false
	}
	message := fmt.Sprintf("target language %q has no matching source files (%s) under %s; its language checks will find nothing",
		target.Language, strings.Join(extensions, ", "), target.Path)
	if len(others) > 0 {
		message += fmt.Sprintf("; found %s files instead — fix the target's language", strings.Join(others, ", "))
	}
	return warnDoctorCheck("language:"+target.Name, message), true
}

// probeTargetLanguages walks root (honouring the scanner's default and
// configured excludes) until a file with one of extensions turns up. It also
// collects the other known source extensions it saw, for the hint. complete is
// false when the walk hit doctorLanguageWalkLimit or an error.
func probeTargetLanguages(root string, extensions []string, excludes []string) (found bool, others []string, complete bool) {
	wanted := make(map[string]struct{}, len(extensions))
	for _, ext := range extensions {
		wanted[ext] = struct{}{}
	}
	known := knownDoctorExtensions()
	seen := map[string]struct{}{}
	visited := 0
	errLimit := errors.New("walk limit reached")
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}
		visited++
		if visited > doctorLanguageWalkLimit {
			return errLimit
		}
		rel, relErr := filepath.Rel(root, current)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(path.Base(rel), ".") || runnersupport.ShouldExclude(rel, excludes) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(path.Ext(rel))
		if _, ok := wanted[ext]; ok {
			found = true
			return fs.SkipAll
		}
		if _, ok := known[ext]; ok {
			seen[ext] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return false, nil, false
	}
	for ext := range seen {
		others = append(others, ext)
	}
	sort.Strings(others)
	return found, others, true
}

func knownDoctorExtensions() map[string]struct{} {
	known := map[string]struct{}{}
	for _, extensions := range doctorLanguageExtensions {
		for _, ext := range extensions {
			known[ext] = struct{}{}
		}
	}
	return known
}
