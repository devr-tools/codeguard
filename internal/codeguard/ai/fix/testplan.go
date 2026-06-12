package fix

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func buildTestPlan(cfg core.Config, patched core.Config, changedByTarget map[string][]string, opts Options) ([]testStep, error) {
	patchedTargets := make(map[string]core.TargetConfig, len(patched.Targets))
	for _, target := range patched.Targets {
		patchedTargets[target.Name] = target
	}

	steps := inferredTestSteps(cfg, patchedTargets, changedByTarget, opts.MaxNearestTests)
	for _, command := range opts.TestCommands {
		step, err := explicitTestStep(patched.Targets, patchedTargets, command)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}

	return dedupeTestSteps(steps), nil
}

func inferredTestSteps(cfg core.Config, patchedTargets map[string]core.TargetConfig, changedByTarget map[string][]string, maxNearest int) []testStep {
	steps := make([]testStep, 0)
	for _, target := range cfg.Targets {
		changed := changedByTarget[target.Name]
		if len(changed) == 0 {
			continue
		}
		patchedTarget, ok := patchedTargets[target.Name]
		if !ok {
			continue
		}
		for _, check := range inferTestCommands(target, patchedTarget.Path, changed, cfg.Exclude, maxNearest) {
			steps = append(steps, testStep{target: patchedTarget, dir: patchedTarget.Path, check: check})
		}
	}
	return steps
}

func explicitTestStep(patchedTargets []core.TargetConfig, targetIndex map[string]core.TargetConfig, command VerificationCommand) (testStep, error) {
	targetName := strings.TrimSpace(command.TargetName)
	if targetName == "" {
		if len(patchedTargets) != 1 {
			return testStep{}, fmt.Errorf("explicit test command %q requires a target_name when multiple targets are configured", command.Check.Name)
		}
		targetName = patchedTargets[0].Name
	}
	patchedTarget, ok := targetIndex[targetName]
	if !ok {
		return testStep{}, fmt.Errorf("explicit test command target %q not found", targetName)
	}
	return testStep{target: patchedTarget, dir: patchedTarget.Path, check: command.Check}, nil
}

func inferTestCommands(target core.TargetConfig, patchedRoot string, changed []string, excludes []string, maxNearest int) []core.CommandCheckConfig {
	switch normalizedLanguage(target.Language) {
	case "", "go":
		return inferGoTestCommands(patchedRoot, changed, excludes, maxNearest)
	default:
		return nil
	}
}

func inferGoTestCommands(root string, changed []string, excludes []string, maxNearest int) []core.CommandCheckConfig {
	testFiles, err := runnersupport.WalkFiles(root, excludes, func(rel string) bool {
		return strings.HasSuffix(rel, "_test.go")
	})
	if err != nil {
		return nil
	}

	selected := nearestOrFallbackGoTests(changed, testFiles, maxNearest)
	checks := make([]core.CommandCheckConfig, 0, len(selected))
	for _, dir := range selected {
		pattern, name := goTestPattern(filepath.ToSlash(dir))
		checks = append(checks, core.CommandCheckConfig{
			Name:    name,
			Command: "go",
			Args:    []string{"test", pattern},
		})
	}
	return checks
}

func nearestOrFallbackGoTests(changed []string, testFiles []string, maxNearest int) []string {
	limit := maxNearest
	if limit <= 0 {
		limit = 3
	}

	selected := nearestGoTestFiles(changed, testFiles, limit)
	if len(selected) == 0 {
		return fallbackGoPackageDirs(changed)
	}
	return uniquePackageDirs(selected)
}

func nearestGoTestFiles(changed []string, testFiles []string, limit int) []string {
	type scoredCandidate struct {
		path  string
		score int
	}

	best := map[string]int{}
	for _, changedFile := range changed {
		for _, testFile := range testFiles {
			score := goTestScore(changedFile, testFile)
			if score <= 0 || score <= best[testFile] {
				continue
			}
			best[testFile] = score
		}
	}
	if len(best) == 0 {
		return nil
	}

	ranked := make([]scoredCandidate, 0, len(best))
	for path, score := range best {
		ranked = append(ranked, scoredCandidate{path: path, score: score})
	}
	slices.SortFunc(ranked, func(a, b scoredCandidate) int {
		if a.score != b.score {
			return b.score - a.score
		}
		return strings.Compare(a.path, b.path)
	})

	if limit > len(ranked) {
		limit = len(ranked)
	}
	selected := make([]string, 0, limit)
	for _, item := range ranked[:limit] {
		selected = append(selected, item.path)
	}
	return selected
}

func goTestScore(changedFile string, testFile string) int {
	if !strings.HasSuffix(changedFile, ".go") || strings.HasSuffix(changedFile, "_test.go") {
		return 0
	}
	changedDir := filepath.ToSlash(filepath.Dir(changedFile))
	testDir := filepath.ToSlash(filepath.Dir(testFile))
	changedBase := strings.TrimSuffix(filepath.Base(changedFile), ".go")
	testBase := strings.TrimSuffix(filepath.Base(testFile), "_test.go")

	score := 10
	if changedDir == testDir {
		score += 100
	}
	if changedBase == testBase {
		score += 60
	}
	if strings.HasPrefix(testBase, changedBase) || strings.HasPrefix(changedBase, testBase) {
		score += 25
	}
	score -= pathDistance(changedDir, testDir) * 5
	return score
}

func fallbackGoPackageDirs(changed []string) []string {
	dirs := make([]string, 0, len(changed))
	for _, rel := range changed {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(rel))
		if dir == "" {
			dir = "."
		}
		dirs = append(dirs, dir)
	}
	return uniquePackageDirs(dirs)
}

func uniquePackageDirs(paths []string) []string {
	seen := map[string]struct{}{}
	dirs := make([]string, 0, len(paths))
	for _, path := range paths {
		dir := filepath.ToSlash(path)
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".go") {
			dir = filepath.ToSlash(filepath.Dir(path))
		}
		if dir == "" {
			dir = "."
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		dirs = append(dirs, dir)
	}
	slices.Sort(dirs)
	return dirs
}

func dedupeTestSteps(steps []testStep) []testStep {
	seen := map[string]struct{}{}
	out := make([]testStep, 0, len(steps))
	for _, step := range steps {
		key := step.target.Name + "\x00" + step.check.Name + "\x00" + step.check.Command + "\x00" + strings.Join(step.check.Args, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, step)
	}
	return out
}
