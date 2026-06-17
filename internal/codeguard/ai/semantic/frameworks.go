package semantic

import "sort"

func detectFrameworks(files []FileSnapshot) []FrameworkRef {
	frameworks := make([]FrameworkRef, 0)
	for _, file := range files {
		signals := expressSignals(file)
		if len(signals) > 0 {
			frameworks = append(frameworks, FrameworkRef{
				Name:    "express",
				Path:    file.Path,
				Signals: signals,
				Hints:   expressHints(file),
			})
		}
		signals = nextJSSignals(file)
		if len(signals) > 0 {
			frameworks = append(frameworks, FrameworkRef{
				Name:    "nextjs",
				Path:    file.Path,
				Signals: signals,
				Hints:   nextJSHints(file),
			})
		}
		signals = reactSignals(file)
		if len(signals) > 0 {
			frameworks = append(frameworks, FrameworkRef{
				Name:    "react",
				Path:    file.Path,
				Signals: signals,
				Hints:   reactHints(file),
			})
		}
	}
	sort.Slice(frameworks, func(i, j int) bool {
		if frameworks[i].Name == frameworks[j].Name {
			return frameworks[i].Path < frameworks[j].Path
		}
		return frameworks[i].Name < frameworks[j].Name
	})
	return frameworks
}
