package performance

import (
	"fmt"
	"io"
	"os"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type budgetMeasurementSpec struct {
	read  func(string) (float64, map[string]float64, error)
	key   string
	label func(float64) string
}

func budgetMeasurementFindings(env support.Context, target core.TargetConfig, budget core.PerformanceBudgetConfig, spec budgetMeasurementSpec) []core.Finding {
	paths, finding := resolveBudgetArtifacts(env, target, budget)
	if finding != nil {
		return []core.Finding{*finding}
	}
	var total float64
	for _, path := range paths {
		totalMillis, keyedMillis, err := spec.read(path)
		if err != nil {
			return []core.Finding{budgetIssueFinding(env, budget, err.Error())}
		}
		if spec.key != "" {
			total += keyedMillis[spec.key]
			continue
		}
		total += totalMillis
	}
	if total <= float64(budget.MaxMilliseconds) {
		return nil
	}
	return []core.Finding{buildTimeExceededFinding(env, budget, spec.label(total))}
}

func readLimitedFile(path string, limit int64, subject string) ([]byte, error) {
	info, err := os.Stat(path) //nolint:gosec // containment verified by caller
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("%s %q is %d bytes, larger than the %d byte limit", subject, path, info.Size(), limit)
	}
	f, err := os.Open(path) //nolint:gosec // containment verified by caller
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(io.LimitReader(f, limit))
}
