package fix

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func changedFilesByTarget(targets []core.TargetConfig, diffText string) map[string][]string {
	changed := make(map[string][]string, len(targets))
	for _, target := range targets {
		rebased := runnersupport.RebaseUnifiedDiff(diffText, runnersupport.DiffPrefixForTarget(target.Path))
		if strings.TrimSpace(rebased) == "" {
			continue
		}
		files := runnersupport.ChangedFilesFromUnifiedDiff(rebased)
		if len(files) == 0 {
			continue
		}
		changed[target.Name] = files
	}
	return changed
}

func flattenChangedFiles(changed map[string][]string) []string {
	seen := map[string]struct{}{}
	files := make([]string, 0)
	for _, rels := range changed {
		for _, rel := range rels {
			if _, ok := seen[rel]; ok {
				continue
			}
			seen[rel] = struct{}{}
			files = append(files, rel)
		}
	}
	slices.Sort(files)
	return files
}

func pathDistance(a string, b string) int {
	a = filepath.ToSlash(strings.TrimSpace(a))
	b = filepath.ToSlash(strings.TrimSpace(b))
	if a == b {
		return 0
	}
	aParts := splitPath(a)
	bParts := splitPath(b)
	common := 0
	for common < len(aParts) && common < len(bParts) && aParts[common] == bParts[common] {
		common++
	}
	return (len(aParts) - common) + (len(bParts) - common)
}

func splitPath(path string) []string {
	if path == "" || path == "." {
		return nil
	}
	return strings.Split(strings.Trim(path, "/"), "/")
}

func joinCommand(check core.CommandCheckConfig) string {
	parts := make([]string, 0, 1+len(check.Args))
	if strings.TrimSpace(check.Command) != "" {
		parts = append(parts, check.Command)
	}
	parts = append(parts, check.Args...)
	return strings.Join(parts, " ")
}

func normalizedLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}

func verificationBaseRef(opts Options) string {
	if strings.TrimSpace(opts.BaseRef) != "" {
		return strings.TrimSpace(opts.BaseRef)
	}
	return "stdin"
}
