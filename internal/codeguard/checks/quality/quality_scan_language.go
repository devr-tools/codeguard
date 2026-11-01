package quality

import (
	"context"
	"go/ast"
	"go/token"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

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
				var index *goPackageIndex
				if localPrecisionEnabled(env) && env.VisitTargetFiles != nil {
					index = buildGoPackageIndex(env, target)
				}
				modules := newGoModuleResolver(target.Path)
				findings := support.ScanTargetFilesWithDependencies(env, target, "quality", isGoSourcePath, goQualityDependencies(index), func(file string, data []byte) []core.Finding {
					return goFindingsForFileWithIndex(env, file, data, index, &modules)
				})
				if localPrecisionEnabled(env) && env.VisitTargetFiles != nil {
					visitTargetFilesInParallel(env, target, func(file string) bool { return strings.HasSuffix(file, ".go") }, func(file string, data []byte) {
						addUnresolved(goUnresolvedMutationEvidenceWithIndex(env, file, data, index))
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
					visitTargetFilesInParallel(env, target, isTypeScriptLikeFile, func(file string, data []byte) {
						addUnresolved(typeScriptUnresolvedMutationEvidence(env, file, data))
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
					env.VisitTargetFiles(target, func(file string) bool { return support.IsCPPPath(file, true) }, func(file string, data []byte) {
						addUnresolved(cppUnresolvedMutationEvidence(env, file, data))
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

// goQualityDependencies declares what the Go quality evaluator reads besides
// the scanned file: go.mod/go.work through the module resolver (import-path
// resolution for quality.dependency-direction), and, when local precision
// built a package-wide index, every sibling .go file in the same directory
// (package types and variables seen by the mutation resolver).
func goQualityDependencies(index *goPackageIndex) support.FileScanDependencies {
	deps := support.FileScanDependencies{SharedFiles: isGoManifestPath}
	if index != nil {
		deps.PackageFiles = isGoSourcePath
	}
	return deps
}

func isGoSourcePath(rel string) bool {
	return strings.HasSuffix(rel, ".go")
}

func isGoManifestPath(rel string) bool {
	base := path.Base(filepath.ToSlash(rel))
	return base == "go.mod" || base == "go.work"
}

func buildGoPackageIndex(env support.Context, target core.TargetConfig) *goPackageIndex {
	index := newGoPackageIndex()
	type parsedSource struct {
		fset   *token.FileSet
		parsed *ast.File
	}
	// Parsing dominates; it is per-file and runs on the bounded pool. addFile
	// lets a later declaration overwrite an earlier one, so the parsed files
	// are folded into the index sequentially in visit order.
	visitTargetFileBatches(env, target, func(file string) bool { return strings.HasSuffix(file, ".go") }, func(batch []visitedFile) {
		parsed := make([]parsedSource, len(batch))
		support.ParallelFor(len(batch), func(i int) {
			if fset, file, err := support.ParseGoSource(env, batch[i].rel, batch[i].data); err == nil {
				parsed[i] = parsedSource{fset: fset, parsed: file}
			}
		})
		for i, source := range parsed {
			if source.parsed != nil {
				index.addFile(batch[i].rel, source.fset, source.parsed)
			}
		}
	})
	return index
}

func goUnresolvedMutationEvidenceWithIndex(env support.Context, file string, data []byte, index *goPackageIndex) []unresolvedMutationEvidence {
	fset, parsed, err := support.ParseGoSource(env, file, data)
	if err != nil {
		return nil
	}
	if index == nil {
		index = newGoPackageIndex()
		index.addFile(file, fset, parsed)
	}
	pkg := index.packageFor(file, parsed.Name.Name)
	provenGlobals := goPackageVariableNames(parsed)
	var unresolved []unresolvedMutationEvidence
	ast.Inspect(parsed, func(node ast.Node) bool {
		declaration, ok := node.(*ast.FuncDecl)
		if !ok {
			return true
		}
		fn := goPrecisionFunction(fset, declaration, data)
		fn.GoFile = file
		fn.GoPackage = pkg
		fn.ProvenGlobals = provenGlobals
		analysis := functionMutationAnalysis(fn, "go")
		unresolved = append(unresolved, analysis.Unresolved...)
		return true
	})
	return unresolved
}

func cppUnresolvedMutationEvidence(env support.Context, file string, data []byte) []unresolvedMutationEvidence {
	parsed := support.ParseCLikeSource(env, file, data, support.CLikeCPP)
	var unresolved []unresolvedMutationEvidence
	for _, fn := range parsed.AllFunctions() {
		analysis := cppFunctionMutationEvidence(parsedPrecisionFunction(fn))
		unresolved = append(unresolved, analysis.Unresolved...)
	}
	return unresolved
}

func typeScriptUnresolvedMutationEvidence(env support.Context, file string, data []byte) []unresolvedMutationEvidence {
	parsed := support.ParseCLikeSource(env, file, data, support.CLikeTypeScript)
	var unresolved []unresolvedMutationEvidence
	for _, fn := range parsedPrecisionFunctions(parsed) {
		analysis := functionMutationAnalysis(fn, "typescript")
		unresolved = append(unresolved, analysis.Unresolved...)
	}
	return unresolved
}
