package buildregression

import "sort"

// Regression records one build command whose duration grew beyond the
// tolerated percentage relative to the stored baseline.
type Regression struct {
	Name                   string
	BaselineDurationMillis float64
	CurrentDurationMillis  float64
	Percent                float64
}

func Compare(baseline map[string]BaselineEntry, current []Result, maxRegressionPercent float64) []Regression {
	regressions := make([]Regression, 0)
	for _, result := range current {
		entry, ok := baseline[result.Name]
		if !ok || entry.DurationMillis <= 0 {
			continue
		}
		percent := (result.DurationMillis - entry.DurationMillis) / entry.DurationMillis * 100
		if percent <= maxRegressionPercent {
			continue
		}
		regressions = append(regressions, Regression{
			Name:                   result.Name,
			BaselineDurationMillis: entry.DurationMillis,
			CurrentDurationMillis:  result.DurationMillis,
			Percent:                percent,
		})
	}
	sort.Slice(regressions, func(i, j int) bool { return regressions[i].Name < regressions[j].Name })
	return regressions
}
