package runner

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/codeguard/checks/ci"
	"github.com/devr-tools/codeguard/codeguard/checks/design"
	"github.com/devr-tools/codeguard/codeguard/checks/languages"
	"github.com/devr-tools/codeguard/codeguard/checks/prompts"
	"github.com/devr-tools/codeguard/codeguard/checks/quality"
	"github.com/devr-tools/codeguard/codeguard/checks/security"
	"github.com/devr-tools/codeguard/codeguard/checks/targets"
	"github.com/devr-tools/codeguard/codeguard/config"
	"github.com/devr-tools/codeguard/codeguard/core"
)

type Runner struct {
	cfg core.Config
}

func New(cfg core.Config) *Runner {
	return &Runner{cfg: cfg}
}

func (r *Runner) Run(ctx context.Context) (core.Report, error) {
	return r.RunWithOptions(ctx, core.ScanOptions{Mode: core.ScanModeFull})
}

func (r *Runner) RunWithOptions(ctx context.Context, opts core.ScanOptions) (core.Report, error) {
	if err := config.Validate(r.cfg); err != nil {
		return core.Report{}, err
	}

	scope, err := r.resolveScope(ctx, opts)
	if err != nil {
		return core.Report{}, err
	}

	sections := []core.SectionResult{
		targets.Evaluate(r.cfg, scope),
		languages.Evaluate(r.cfg, scope),
		quality.Evaluate(r.cfg, scope),
		design.Evaluate(r.cfg, scope),
		security.Evaluate(r.cfg, scope),
		prompts.Evaluate(r.cfg, scope),
		ci.Evaluate(r.cfg, scope),
	}

	return core.Report{
		Name:        r.cfg.Name,
		GeneratedAt: time.Now().UTC(),
		ScanMode:    scope.Mode,
		BaseRef:     scope.BaseRef,
		Sections:    sections,
		Summary:     summarize(sections),
	}, nil
}

func (r *Runner) resolveScope(ctx context.Context, opts core.ScanOptions) (core.ScanScope, error) {
	mode := opts.Mode
	if mode == "" {
		mode = core.ScanModeFull
	}
	scope := core.ScanScope{Mode: mode}
	if mode != core.ScanModeDiff {
		return scope, nil
	}

	baseRef := strings.TrimSpace(opts.BaseRef)
	if baseRef == "" {
		baseRef = "main"
	}
	scope.BaseRef = baseRef

	workDir, err := r.resolveWorkDir()
	if err != nil {
		return core.ScanScope{}, err
	}
	changed := make(map[string]struct{})
	commands := [][]string{
		{"diff", "--name-only", baseRef + "...HEAD"},
		{"diff", "--name-only"},
		{"diff", "--name-only", "--cached"},
	}
	for _, args := range commands {
		out, err := exec.CommandContext(ctx, "git", append([]string{"-C", workDir}, args...)...).Output()
		if err != nil {
			return core.ScanScope{}, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			changed[filepath.Clean(filepath.Join(workDir, line))] = struct{}{}
		}
	}
	scope.ChangedFiles = changed
	return scope, nil
}

func (r *Runner) resolveWorkDir() (string, error) {
	if len(r.cfg.Targets) == 0 {
		return filepath.Abs(".")
	}
	return filepath.Abs(r.cfg.Targets[0].Path)
}

func summarize(sections []core.SectionResult) core.Summary {
	var summary core.Summary
	for _, section := range sections {
		switch section.Status {
		case core.StatusPass:
			summary.PassedSections++
		case core.StatusWarn:
			summary.WarnedSections++
		case core.StatusFail:
			summary.FailedSections++
		case core.StatusSkip:
			summary.SkippedSections++
		}
	}
	return summary
}
