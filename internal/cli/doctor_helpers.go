package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

type doctorCheck struct {
	Name    string
	Status  string
	Message string
}

func gitDoctorCheck() doctorCheck {
	if _, err := exec.LookPath("git"); err != nil {
		return failDoctorCheck("git", "git is not available on PATH")
	}
	return passDoctorCheck("git", "git is available")
}

func targetDoctorChecks(targets []service.TargetConfig) []doctorCheck {
	checks := make([]doctorCheck, 0, len(targets)*2)
	for _, target := range targets {
		if !targetPathExists(target.Path) {
			checks = append(checks, failDoctorCheck("target:"+target.Name, fmt.Sprintf("target path %s is missing", target.Path)))
			continue
		}

		checks = append(checks, passDoctorCheck("target:"+target.Name, fmt.Sprintf("target path %s exists", target.Path)))
		checks = append(checks, repoDoctorCheck(target))
	}
	return checks
}

func repoDoctorCheck(target service.TargetConfig) doctorCheck {
	if err := exec.Command("git", "-C", target.Path, "rev-parse", "--show-toplevel").Run(); err != nil {
		return warnDoctorCheck("repo:"+target.Name, fmt.Sprintf("%s is not a git worktree; diff scans will not work", target.Path))
	}
	return passDoctorCheck("repo:"+target.Name, "git worktree detected")
}

func baselineDoctorCheck(cfg service.Config) (doctorCheck, bool) {
	if cfg.Baseline.Path == "" {
		return doctorCheck{}, false
	}
	if _, err := os.Stat(cfg.Baseline.Path); err != nil {
		return warnDoctorCheck("baseline", fmt.Sprintf("baseline file %s is missing", cfg.Baseline.Path)), true
	}
	return passDoctorCheck("baseline", "baseline file found"), true
}

func cacheDoctorCheck(cfg service.Config) (doctorCheck, bool) {
	if cfg.Cache.Enabled == nil || !*cfg.Cache.Enabled {
		return doctorCheck{}, false
	}

	cacheDir := filepath.Dir(cfg.Cache.Path)
	if cacheDir == "" {
		cacheDir = "."
	}
	if _, err := os.Stat(cacheDir); err != nil {
		if os.IsNotExist(err) {
			return passDoctorCheck("cache", fmt.Sprintf("cache directory %s will be created on first run", cacheDir)), true
		}
		return warnDoctorCheck("cache", fmt.Sprintf("cache directory %s is not writable", cacheDir)), true
	}
	return passDoctorCheck("cache", fmt.Sprintf("cache will be written to %s", cfg.Cache.Path)), true
}

func targetPathExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
