package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/runner/buildregression"
	"github.com/devr-tools/codeguard/internal/codeguard/trust"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func buildRegressionConfig(name string, dir string, build codeguard.PerformanceBuildRegressionConfig) codeguard.Config {
	cfg := performanceConfig(name, dir, "typescript")
	cfg.Checks.PerformanceRules.BuildRegression = build
	return cfg
}

func TestBuildRegressionComparator(t *testing.T) {
	baseline := map[string]buildregression.BaselineEntry{
		"repo:web-build":    {DurationMillis: 1000},
		"repo:slow-build":   {DurationMillis: 1000},
		"repo:faster-build": {DurationMillis: 1000},
		"repo:zero-build":   {DurationMillis: 0},
	}
	current := []buildregression.Result{
		{Name: "repo:web-build", DurationMillis: 1100},
		{Name: "repo:slow-build", DurationMillis: 1500},
		{Name: "repo:faster-build", DurationMillis: 500},
		{Name: "repo:zero-build", DurationMillis: 100},
		{Name: "repo:new-build", DurationMillis: 9999},
	}
	regressions := buildregression.Compare(baseline, current, 20)
	if len(regressions) != 1 {
		t.Fatalf("got %d regressions, want 1: %+v", len(regressions), regressions)
	}
	got := regressions[0]
	if got.Name != "repo:slow-build" || got.Percent != 50 || got.BaselineDurationMillis != 1000 || got.CurrentDurationMillis != 1500 {
		t.Fatalf("regression fields wrong: %+v", got)
	}
}

func TestBuildRegressionBaselineRoundTripAndMerge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.build-baseline.json")
	results := []buildregression.Result{{Name: "repo:web-build", DurationMillis: 123}}
	if err := buildregression.WriteBaseline(path, results); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	baseline, ok := buildregression.LoadBaseline(path)
	if !ok || baseline["repo:web-build"].DurationMillis != 123 {
		t.Fatalf("baseline roundtrip failed: ok=%v %+v", ok, baseline)
	}

	added, err := buildregression.MergeNewCommands(path, baseline, []buildregression.Result{
		{Name: "repo:web-build", DurationMillis: 999},
		{Name: "repo:assets-build", DurationMillis: 50},
	})
	if err != nil || !added {
		t.Fatalf("merge: added=%v err=%v", added, err)
	}
	merged, ok := buildregression.LoadBaseline(path)
	if !ok || merged["repo:web-build"].DurationMillis != 123 || merged["repo:assets-build"].DurationMillis != 50 {
		t.Fatalf("merge result wrong: %+v", merged)
	}

	if _, ok := buildregression.LoadBaseline(filepath.Join(t.TempDir(), "missing.json")); ok {
		t.Fatal("missing baseline unexpectedly loaded")
	}
}

func TestBuildRegressionBaselinePathDerivedFromCachePath(t *testing.T) {
	if got := buildregression.BaselinePathForBase(".codeguard/cache.json"); got != ".codeguard/cache.build-baseline.json" {
		t.Fatalf("derived baseline path = %q", got)
	}
	if got := buildregression.BaselinePathForBase(""); got != "" {
		t.Fatalf("empty base should derive empty path, got %q", got)
	}
}

func TestPerformanceBuildRegressionValidation(t *testing.T) {
	dir := t.TempDir()
	enabled := true
	cases := []struct {
		label string
		cfg   codeguard.PerformanceBuildRegressionConfig
		want  string
	}{
		{
			label: "enabled without commands",
			cfg:   codeguard.PerformanceBuildRegressionConfig{Enabled: &enabled},
			want:  "must list at least one command",
		},
		{
			label: "negative threshold",
			cfg: codeguard.PerformanceBuildRegressionConfig{
				Commands:             []codeguard.CommandCheckConfig{{Name: "build", Command: "make"}},
				MaxRegressionPercent: -1,
			},
			want: "must not be negative",
		},
		{
			label: "empty command name",
			cfg: codeguard.PerformanceBuildRegressionConfig{
				Commands: []codeguard.CommandCheckConfig{{Command: "make"}},
			},
			want: ".name is required",
		},
		{
			label: "empty command binary",
			cfg: codeguard.PerformanceBuildRegressionConfig{
				Commands: []codeguard.CommandCheckConfig{{Name: "build"}},
			},
			want: ".command is required",
		},
		{
			label: "duplicate names",
			cfg: codeguard.PerformanceBuildRegressionConfig{
				Commands: []codeguard.CommandCheckConfig{
					{Name: "build", Command: "make"},
					{Name: "build", Command: "npm"},
				},
			},
			want: "duplicates another build regression command name",
		},
	}
	for _, tc := range cases {
		cfg := buildRegressionConfig("build-regression-validate", dir, tc.cfg)
		err := codeguard.ValidateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: expected error containing %q, got %v", tc.label, tc.want, err)
		}
	}
}

func TestPerformanceBuildRegressionBlockedWhenConfigCommandsDisabled(t *testing.T) {
	t.Setenv("CODEGUARD_ALLOW_CONFIG_COMMANDS", "")
	trust.ResetFromEnv()
	t.Cleanup(trust.ResetFromEnv)
	dir := t.TempDir()
	script := filepath.Join(dir, "build.sh")
	writeExecutableFile(t, script, "#!/bin/sh\nexit 0\n")

	enabled := true
	report, err := codeguard.Run(context.Background(), buildRegressionConfig("build-regression-trust", dir, codeguard.PerformanceBuildRegressionConfig{
		Enabled:      &enabled,
		Commands:     []codeguard.CommandCheckConfig{{Name: "build", Command: script}},
		BaselinePath: filepath.Join(dir, ".codeguard", "build-baseline.json"),
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	assertFindingRulePresent(t, report, "Performance", "performance.build-regression")
	assertFindingMessageContains(t, report, "performance.build-regression", "refusing to run config-supplied command")
}

func TestPerformanceBuildRegressionEndToEnd(t *testing.T) {
	t.Setenv("CODEGUARD_ALLOW_CONFIG_COMMANDS", "1")
	trust.ResetFromEnv()
	t.Cleanup(trust.ResetFromEnv)
	dir := t.TempDir()
	script := filepath.Join(dir, "build.sh")
	writeExecutableFile(t, script, "#!/bin/sh\nsleep 0.05\n")
	baselinePath := filepath.Join(dir, ".codeguard", "build-baseline.json")
	enabled := true
	cfg := buildRegressionConfig("build-regression-e2e", dir, codeguard.PerformanceBuildRegressionConfig{
		Enabled:      &enabled,
		Commands:     []codeguard.CommandCheckConfig{{Name: "build", Command: script}},
		BaselinePath: baselinePath,
	})

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.build-regression")
	if _, statErr := os.Stat(baselinePath); statErr != nil {
		t.Fatalf("first run did not write the baseline: %v", statErr)
	}

	writeFile(t, baselinePath, `{"version": 1, "commands": {"repo:build": {"duration_millis": 0.01}}}`)
	report, err = codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	assertFindingMessageContains(t, report, "performance.build-regression", "repo:build regressed")
}
