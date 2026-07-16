package performance

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func scanLanguagePerformanceFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return support.DispatchByLanguage(target.Language,
		support.LanguageDispatch{
			Aliases: []string{"", "go"},
			Run: func() []core.Finding {
				findings := goRebuildCascadeFindings(env, target)
				return append(findings, support.ScanGoFiles(env, target, "performance", func(file string, data []byte) []core.Finding {
					return goFindingsForFile(env, file, data)
				})...)
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"python", "py"},
			Run: func() []core.Finding {
				return support.ScanPythonFiles(env, target, "performance", func(file string, data []byte) []core.Finding {
					return pythonPerformanceFindings(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"rust", "rs"},
			Run: func() []core.Finding {
				return support.ScanRustFiles(env, target, "performance", func(file string, data []byte) []core.Finding {
					return rustPerformanceFindings(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"c++", "cpp", "cxx"},
			Run: func() []core.Finding {
				return support.ScanCPPFiles(env, target, "performance", func(file string, data []byte) []core.Finding {
					return cppPerformanceFindings(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"typescript", "javascript", "ts", "tsx", "js", "jsx"},
			Run: func() []core.Finding {
				return typeScriptPerformanceTargetFindings(env, target)
			},
		},
	)
}
