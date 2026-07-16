package performance

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const maxTimeTraceFileBytes = 32 << 20 // 32 MiB

type clangTimeTrace struct {
	totalMillis float64
	events      map[string]float64
}

func clangTimeTraceBudgetFindings(env support.Context, target core.TargetConfig, budget core.PerformanceBudgetConfig) []core.Finding {
	paths, finding := resolveBudgetArtifacts(env, target, budget)
	if finding != nil {
		return []core.Finding{*finding}
	}
	var total float64
	for _, path := range paths {
		trace, err := readClangTimeTrace(path)
		if err != nil {
			return []core.Finding{budgetIssueFinding(env, budget, fmt.Sprintf("time trace %q: %v; budget skipped", budget.Path, err))}
		}
		if budget.Event != "" {
			total += trace.events[budget.Event]
			continue
		}
		total += trace.totalMillis
	}
	if total <= float64(budget.MaxMilliseconds) {
		return nil
	}
	if budget.Event != "" {
		return []core.Finding{buildTimeExceededFinding(env, budget, fmt.Sprintf("events named %q total %.1f ms", budget.Event, total))}
	}
	return []core.Finding{buildTimeExceededFinding(env, budget, fmt.Sprintf("%q totals %.1f ms", budget.Path, total))}
}

func readClangTimeTrace(path string) (clangTimeTrace, error) {
	info, err := os.Stat(path) //nolint:gosec // containment verified by caller
	if err != nil {
		return clangTimeTrace{}, err
	}
	if info.Size() > maxTimeTraceFileBytes {
		return clangTimeTrace{}, fmt.Errorf("time trace is %d bytes, larger than the %d byte limit", info.Size(), maxTimeTraceFileBytes)
	}
	f, err := os.Open(path) //nolint:gosec // containment verified by caller
	if err != nil {
		return clangTimeTrace{}, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxTimeTraceFileBytes))
	if err != nil {
		return clangTimeTrace{}, err
	}
	return parseClangTimeTrace(data)
}

func parseClangTimeTrace(data []byte) (clangTimeTrace, error) {
	var payload struct {
		TraceEvents []struct {
			Name string  `json:"name"`
			PH   string  `json:"ph"`
			TS   float64 `json:"ts"`
			Dur  float64 `json:"dur"`
		} `json:"traceEvents"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return clangTimeTrace{}, err
	}
	if len(payload.TraceEvents) == 0 {
		return clangTimeTrace{}, fmt.Errorf("traceEvents array is empty")
	}
	trace := clangTimeTrace{events: map[string]float64{}}
	var minTS, maxEnd float64
	var haveSpan bool
	for _, event := range payload.TraceEvents {
		if event.PH != "X" || event.Dur <= 0 {
			continue
		}
		if !haveSpan || event.TS < minTS {
			minTS = event.TS
		}
		if end := event.TS + event.Dur; !haveSpan || end > maxEnd {
			maxEnd = end
		}
		haveSpan = true
		if event.Name != "" {
			trace.events[event.Name] += microsToMillis(event.Dur)
		}
	}
	if !haveSpan {
		return clangTimeTrace{}, fmt.Errorf("no complete duration events found in traceEvents")
	}
	trace.totalMillis = microsToMillis(maxEnd - minTS)
	return trace, nil
}

func microsToMillis(micros float64) float64 {
	return micros / 1000.0
}

func buildTimeExceededFinding(env support.Context, budget core.PerformanceBudgetConfig, measurement string) core.Finding {
	return performanceBudgetLimitFinding(env, budget, measurement, "max_milliseconds", budget.MaxMilliseconds)
}
