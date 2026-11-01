package benchregression

import "sort"

// Regression records one benchmark whose ns/op grew beyond the tolerated
// percentage relative to the stored baseline.
type Regression struct {
	Name            string
	BaselineNsPerOp float64
	CurrentNsPerOp  float64
	Percent         float64
}

// Compare returns the benchmarks in current whose ns/op regressed more than
// maxRegressionPercent against the baseline, sorted by name for deterministic
// output. Benchmarks absent from the baseline (new benchmarks) and baseline
// entries with a non-positive ns/op are skipped: there is nothing meaningful
// to compare against.
func Compare(baseline map[string]BaselineEntry, current []Result, maxRegressionPercent float64) []Regression {
	regressions := make([]Regression, 0)
	for _, result := range current {
		entry, ok := baseline[result.Name]
		if !ok || entry.NsPerOp <= 0 {
			continue
		}
		percent := (result.NsPerOp - entry.NsPerOp) / entry.NsPerOp * 100
		if percent <= maxRegressionPercent {
			continue
		}
		regressions = append(regressions, Regression{
			Name:            result.Name,
			BaselineNsPerOp: entry.NsPerOp,
			CurrentNsPerOp:  result.NsPerOp,
			Percent:         percent,
		})
	}
	sort.Slice(regressions, func(i, j int) bool { return regressions[i].Name < regressions[j].Name })
	return regressions
}
