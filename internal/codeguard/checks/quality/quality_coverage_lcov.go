package quality

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// commandCoverageProfile runs the configured coverage command for a non-Go
// target and parses the lcov report it produces.
func commandCoverageProfile(ctx context.Context, env support.Context, target core.TargetConfig, command core.CoverageCommandConfig) (coverageProfile, error) {
	name := command.Name
	if name == "" {
		name = "coverage"
	}
	output, err := env.RunCommandCheck(ctx, target.Path, core.CommandCheckConfig{
		Name:    name,
		Command: command.Command,
		Args:    command.Args,
	})
	if err != nil {
		if strings.TrimSpace(output) != "" {
			return nil, fmt.Errorf("%s: %s", name, output)
		}
		return nil, err
	}
	reportPath := command.ReportPath
	if !filepath.IsAbs(reportPath) {
		reportPath = filepath.Join(target.Path, reportPath)
	}
	data, err := os.ReadFile(reportPath) //nolint:gosec // config-supplied coverage report path
	if err != nil {
		return nil, fmt.Errorf("coverage report %q: %w", command.ReportPath, err)
	}
	return normalizeProfilePaths(ParseLCOV(string(data)), target.Path), nil
}

// ParseLCOV parses lcov tracefile content into per-line hit counts keyed by
// the SF source path. Only SF, DA, and end_of_record entries are consumed;
// everything else in the format is summary data this check does not need.
func ParseLCOV(data string) map[string]map[int]int {
	profile := map[string]map[int]int{}
	current := ""
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "SF:"):
			current = strings.ReplaceAll(strings.TrimSpace(strings.TrimPrefix(line, "SF:")), "\\", "/")
			if current != "" && profile[current] == nil {
				profile[current] = map[int]int{}
			}
		case strings.HasPrefix(line, "DA:") && current != "":
			recordLCOVLine(profile[current], strings.TrimPrefix(line, "DA:"))
		case line == "end_of_record":
			current = ""
		}
	}
	return profile
}

// recordLCOVLine parses a "DA:<line>,<hits>[,<checksum>]" payload.
func recordLCOVLine(hits map[int]int, payload string) {
	parts := strings.Split(payload, ",")
	if len(parts) < 2 {
		return
	}
	line, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || line <= 0 {
		return
	}
	count, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return
	}
	if existing, ok := hits[line]; !ok || count > existing {
		hits[line] = count
	}
}

// normalizeProfilePaths rewrites absolute lcov source paths to be relative to
// the target so they line up with git diff paths.
func normalizeProfilePaths(parsed map[string]map[int]int, targetPath string) coverageProfile {
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		absTarget = targetPath
	}
	profile := coverageProfile{}
	for source, hits := range parsed {
		rel := source
		if filepath.IsAbs(filepath.FromSlash(source)) {
			if relative, err := filepath.Rel(absTarget, filepath.FromSlash(source)); err == nil && !strings.HasPrefix(relative, "..") {
				rel = filepath.ToSlash(relative)
			}
		}
		profile[rel] = hits
	}
	return profile
}
