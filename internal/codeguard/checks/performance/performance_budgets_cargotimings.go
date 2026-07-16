package performance

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const maxCargoTimingsFileBytes = 32 << 20 // 32 MiB

var cargoTimingsUnitDataPattern = regexp.MustCompile(`(?s)\bUNIT_DATA\s*=\s*(\[[^;]*\])\s*;`)

type cargoTimingsReport struct {
	totalMillis float64
	crateMillis map[string]float64
}

func cargoTimingsBudgetFindings(env support.Context, target core.TargetConfig, budget core.PerformanceBudgetConfig) []core.Finding {
	return budgetMeasurementFindings(env, target, budget, budgetMeasurementSpec{
		read: readCargoTimingsReport,
		key:  budget.Crate,
		label: func(total float64) string {
			if budget.Crate != "" {
				return fmt.Sprintf("crate %q totals %.1f ms", budget.Crate, total)
			}
			return fmt.Sprintf("%q totals %.1f ms", budget.Path, total)
		},
	})
}

func readCargoTimings(path string) (cargoTimingsReport, error) {
	data, err := readLimitedFile(path, maxCargoTimingsFileBytes, "cargo timings report")
	if err != nil {
		return cargoTimingsReport{}, err
	}
	return parseCargoTimings(data)
}

func readCargoTimingsReport(path string) (float64, map[string]float64, error) {
	report, err := readCargoTimings(path)
	if err != nil {
		return 0, nil, fmt.Errorf("cargo timings report %q: %v; budget skipped", path, err)
	}
	return report.totalMillis, report.crateMillis, nil
}

type cargoTimingsUnit struct {
	Name     string  `json:"name"`
	Start    float64 `json:"start"`
	Duration float64 `json:"duration"`
}

func parseCargoTimings(data []byte) (cargoTimingsReport, error) {
	match := cargoTimingsUnitDataPattern.FindSubmatch(data)
	if len(match) != 2 {
		return cargoTimingsReport{}, fmt.Errorf("UNIT_DATA payload not found in cargo timings HTML")
	}
	var units []cargoTimingsUnit
	if err := json.Unmarshal(match[1], &units); err != nil {
		return cargoTimingsReport{}, fmt.Errorf("decode UNIT_DATA: %w", err)
	}
	if len(units) == 0 {
		return cargoTimingsReport{}, fmt.Errorf("UNIT_DATA array is empty")
	}
	report := cargoTimingsReport{crateMillis: make(map[string]float64)}
	var maxEnd float64
	var haveSpan bool
	for _, unit := range units {
		if strings.TrimSpace(unit.Name) == "" || unit.Duration <= 0 {
			continue
		}
		end := unit.Start + unit.Duration
		if !haveSpan || end > maxEnd {
			maxEnd = end
		}
		haveSpan = true
		report.crateMillis[unit.Name] += secondsToMillis(unit.Duration)
	}
	if !haveSpan {
		return cargoTimingsReport{}, fmt.Errorf("no timed units found in UNIT_DATA")
	}
	report.totalMillis = secondsToMillis(maxEnd)
	return report, nil
}

func secondsToMillis(seconds float64) float64 {
	return seconds * 1000.0
}
