package quality

import (
	"context"
	"go/ast"
	"strings"
	"sync"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func languageQualityFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return languageQualityAnalysis(ctx, env, target).findings
}

type languageQualityScan struct {
	findings   []core.Finding
	unresolved []unresolvedMutationEvidence
}

func languageQualityAnalysis(ctx context.Context, env support.Context, target core.TargetConfig) languageQualityScan {
	var unresolvedMu sync.Mutex
	var unresolved []unresolvedMutationEvidence
	addUnresolved := func(items []unresolvedMutationEvidence) {
		if len(items) == 0 {
			return
		}
		unresolvedMu.Lock()
		unresolved = append(unresolved, items...)
		unresolvedMu.Unlock()
	}
	findings := support.DispatchByLanguage(target.Language,
		support.LanguageDispatch{
			Aliases: []string{"", "go"},
			Run: func() []core.Finding {
				findings := support.ScanGoFiles(env, target, "quality", func(file string, data []byte) []core.Finding {
					return goFindingsForFile(env, file, data)
				})
				if localPrecisionEnabled(env) && env.VisitTargetFiles != nil {
					env.VisitTargetFiles(target, func(file string) bool { return strings.HasSuffix(file, ".go") }, func(file string, data []byte) {
						addUnresolved(goUnresolvedMutationEvidence(env, file, data))
					})
				}
				return findings
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
				findings := typeScriptTargetFindings(ctx, env, target)
				if localPrecisionEnabled(env) && env.VisitTargetFiles != nil {
					env.VisitTargetFiles(target, isTypeScriptLikeFile, func(_ string, data []byte) {
						addUnresolved(typeScriptUnresolvedMutationEvidence(data))
					})
				}
				return findings
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
				findings := support.ScanCPPFiles(env, target, "quality", func(file string, data []byte) []core.Finding {
					return cppFindingsForFile(env, file, data)
				})
				if localPrecisionEnabled(env) && env.VisitTargetFiles != nil {
					env.VisitTargetFiles(target, func(file string) bool { return support.IsCPPPath(file, true) }, func(_ string, data []byte) {
						addUnresolved(cppUnresolvedMutationEvidence(data))
					})
				}
				return findings
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
	return languageQualityScan{findings: findings, unresolved: unresolved}
}

func goUnresolvedMutationEvidence(env support.Context, file string, data []byte) []unresolvedMutationEvidence {
	fset, parsed, err := support.ParseGoSource(env, file, data)
	if err != nil {
		return nil
	}
	provenGlobals := goPackageVariableNames(parsed)
	var unresolved []unresolvedMutationEvidence
	ast.Inspect(parsed, func(node ast.Node) bool {
		declaration, ok := node.(*ast.FuncDecl)
		if !ok {
			return true
		}
		fn := goPrecisionFunction(fset, declaration, data)
		fn.ProvenGlobals = provenGlobals
		analysis := functionMutationAnalysis(fn, "go")
		unresolved = append(unresolved, analysis.Unresolved...)
		return true
	})
	return unresolved
}

func cppUnresolvedMutationEvidence(data []byte) []unresolvedMutationEvidence {
	parsed := support.ParseCLike(string(data), support.CLikeCPP)
	var unresolved []unresolvedMutationEvidence
	for _, fn := range parsed.AllFunctions() {
		analysis := functionMutationAnalysis(parsedPrecisionFunction(fn), "c++")
		unresolved = append(unresolved, analysis.Unresolved...)
	}
	return unresolved
}

func typeScriptUnresolvedMutationEvidence(data []byte) []unresolvedMutationEvidence {
	parsed := support.ParseCLike(string(data), support.CLikeTypeScript)
	var unresolved []unresolvedMutationEvidence
	for _, fn := range parsed.AllFunctions() {
		analysis := functionMutationAnalysis(parsedPrecisionFunction(fn), "typescript")
		unresolved = append(unresolved, analysis.Unresolved...)
	}
	return unresolved
}
