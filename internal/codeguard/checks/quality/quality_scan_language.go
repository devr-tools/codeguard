package quality

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func languageQualityFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return support.DispatchByLanguage(target.Language,
		support.LanguageDispatch{
			Aliases: []string{"", "go"},
			Run: func() []core.Finding {
				return support.ScanGoFiles(env, target, "quality", func(file string, data []byte) []core.Finding {
					return goFindingsForFile(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"python", "py"},
			Run: func() []core.Finding {
				return support.ScanPythonFiles(env, target, "quality", func(file string, data []byte) []core.Finding {
					return pythonFindingsForFile(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"typescript", "javascript", "ts", "tsx", "js", "jsx"},
			Run: func() []core.Finding {
				return typeScriptTargetFindings(ctx, env, target)
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"rust", "rs"},
			Run: func() []core.Finding {
				return support.ScanRustFiles(env, target, "quality", func(file string, data []byte) []core.Finding {
					return rustFindingsForFile(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"c++", "cpp", "cxx", "cc"},
			Run: func() []core.Finding {
				return support.ScanCPPFiles(env, target, "quality", func(file string, data []byte) []core.Finding {
					return cppFindingsForFile(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"java"},
			Run: func() []core.Finding {
				return env.ScanTargetFiles(target, "quality", isJavaFile, func(file string, data []byte) []core.Finding {
					return javaFindingsForFile(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"csharp", "c#", "cs", "dotnet"},
			Run: func() []core.Finding {
				return env.ScanTargetFiles(target, "quality", isCSharpFile, func(file string, data []byte) []core.Finding {
					return csharpFindingsForFile(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"ruby", "rb"},
			Run: func() []core.Finding {
				return env.ScanTargetFiles(target, "quality", isRubyFile, func(file string, data []byte) []core.Finding {
					return rubyFindingsForFile(env, file, data)
				})
			},
		},
	)
}
