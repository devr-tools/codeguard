package performance

import (
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func scanLanguagePerformanceFindings(env support.Context, target core.TargetConfig) []core.Finding {
	return support.DispatchByLanguage(target.Language,
		support.LanguageDispatch{
			Aliases: []string{"", "go"},
			Run: func() []core.Finding {
				findings := goRebuildCascadeFindings(env, target)
				return append(findings, support.ScanGoFiles(env, target, support.CacheSectionPerformanceGo, func(file string, data []byte) []core.Finding {
					return goFindingsForFile(env, file, data)
				})...)
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"python", "py"},
			Run: func() []core.Finding {
				return support.ScanPythonFiles(env, target, support.CacheSectionPerformancePython, func(file string, data []byte) []core.Finding {
					return pythonPerformanceFindings(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"rust", "rs"},
			Run: func() []core.Finding {
				return support.ScanRustFiles(env, target, support.CacheSectionPerformanceRust, func(file string, data []byte) []core.Finding {
					return rustPerformanceFindings(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"c++", "cpp", "cxx", "cc"},
			Run: func() []core.Finding {
				findings := cppRebuildCascadeFindings(env, target)
				return append(findings, support.ScanCPPFiles(env, target, support.CacheSectionPerformanceCPP, func(file string, data []byte) []core.Finding {
					return cppPerformanceFindings(env, file, data)
				})...)
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
