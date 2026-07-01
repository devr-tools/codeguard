package quality

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// goCoverageProfile runs `go test -coverprofile` for the packages containing
// changed Go files and returns per-line hit counts keyed by target-relative
// path. It returns a nil profile when no non-test Go files changed.
func goCoverageProfile(ctx context.Context, env support.Context, target core.TargetConfig, scope map[string]core.ChangedLineRanges) (coverageProfile, error) {
	packages := changedGoPackages(scope)
	if len(packages) == 0 {
		return nil, nil
	}
	profileFile, err := os.CreateTemp("", "codeguard-coverage-*.out")
	if err != nil {
		return nil, err
	}
	profilePath := profileFile.Name()
	_ = profileFile.Close()
	defer func() { _ = os.Remove(profilePath) }()

	args := append([]string{"test", "-count=1", "-coverprofile", profilePath}, packages...)
	output, err := env.RunCommandCheck(ctx, target.Path, core.CommandCheckConfig{
		Name:    "go-coverage",
		Command: "go",
		Args:    args,
	})
	if err != nil {
		if strings.TrimSpace(output) != "" {
			return nil, fmt.Errorf("go test: %s", output)
		}
		return nil, fmt.Errorf("go test: %w", err)
	}

	data, err := os.ReadFile(profilePath) //nolint:gosec // config-supplied coverage profile path
	if err != nil {
		return nil, err
	}
	return parseGoCoverProfile(string(data), support.GoModulePath(target.Path)), nil
}

func changedGoPackages(scope map[string]core.ChangedLineRanges) []string {
	dirs := map[string]struct{}{}
	for rel := range scope {
		slashPath := filepath.ToSlash(rel)
		if !strings.HasSuffix(slashPath, ".go") || strings.HasSuffix(slashPath, "_test.go") {
			continue
		}
		dirs["./"+path.Dir(slashPath)] = struct{}{}
	}
	packages := make([]string, 0, len(dirs))
	for dir := range dirs {
		packages = append(packages, dir)
	}
	sort.Strings(packages)
	return packages
}

// parseGoCoverProfile converts a Go cover profile into per-line hit counts.
// Profile blocks look like "module/path/file.go:2.13,4.2 1 1".
func parseGoCoverProfile(data string, modulePath string) coverageProfile {
	profile := coverageProfile{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		colon := strings.LastIndex(line, ":")
		if colon <= 0 {
			continue
		}
		rel := goProfileRelPath(line[:colon], modulePath)
		startLine, endLine, count, ok := parseGoProfileBlock(line[colon+1:])
		if !ok {
			continue
		}
		hits := profile[rel]
		if hits == nil {
			hits = map[int]int{}
			profile[rel] = hits
		}
		for at := startLine; at <= endLine; at++ {
			if existing, ok := hits[at]; !ok || count > existing {
				hits[at] = count
			}
		}
	}
	return profile
}

// parseGoProfileBlock parses "2.13,4.2 1 1" into start line, end line, count.
func parseGoProfileBlock(block string) (int, int, int, bool) {
	fields := strings.Fields(block)
	if len(fields) != 3 {
		return 0, 0, 0, false
	}
	positions := strings.Split(fields[0], ",")
	if len(positions) != 2 {
		return 0, 0, 0, false
	}
	startLine, err := strconv.Atoi(strings.SplitN(positions[0], ".", 2)[0])
	if err != nil {
		return 0, 0, 0, false
	}
	endLine, err := strconv.Atoi(strings.SplitN(positions[1], ".", 2)[0])
	if err != nil {
		return 0, 0, 0, false
	}
	count, err := strconv.Atoi(fields[2])
	if err != nil {
		return 0, 0, 0, false
	}
	return startLine, endLine, count, true
}

func goProfileRelPath(file string, modulePath string) string {
	if modulePath != "" {
		return strings.TrimPrefix(file, modulePath+"/")
	}
	return file
}
