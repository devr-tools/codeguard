package support

import "github.com/devr-tools/codeguard/internal/codeguard/core"

type LanguageDispatch struct {
	Aliases []string
	Run     func() []core.Finding
}

func DispatchByLanguage(language string, cases ...LanguageDispatch) []core.Finding {
	normalized := NormalizedLanguage(language)
	for _, item := range cases {
		for _, alias := range item.Aliases {
			if normalized == alias {
				return item.Run()
			}
		}
	}
	return nil
}
