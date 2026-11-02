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
				findings := support.ScanTargetFilesWithDependencies(env, target, support.CacheSectionQualityGo, isGoSourcePath, goQualityDependencies(target.Path, index), func(file string, data []byte) []core.Finding {
					return goFindingsForFileWithIndex(env, file, data, index, &modules)
				})
				if localPrecisionEnabled(env) && env.VisitTargetFiles != nil {
					support.VisitTargetFilesOrdered(env, target, func(file string) bool { return strings.HasSuffix(file, ".go") }, func(file string, data []byte) func() {
						items := goUnresolvedMutationEvidenceWithIndex(env, file, data, index)
						return func() { addUnresolved(items) }
					})
				}
				return findings
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"python", "py"},
			Run: func() []core.Finding {
				return support.ScanPythonFiles(env, target, support.CacheSectionQualityPython, func(file string, data []byte) []core.Finding {
					return pythonFindingsForFile(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"typescript", "javascript", "ts", "tsx", "js", "jsx"},
			Run: func() []core.Finding {
				var findings []core.Finding
				targetFindings := func() { findings = typeScriptTargetFindings(ctx, env, target) }
				if !localPrecisionEnabled(env) || env.VisitTargetFiles == nil {
					targetFindings()
					return findings
				}
				// The unresolved-mutation pass does not depend on the target
				// findings, and typeScriptTargetFindings spends most of its wall
				// time waiting on the TypeScript semantic runner subprocess, so
				// the two overlap. This dispatch branch is the only producer of
				// unresolved evidence, so the order addUnresolved sees is
				// unchanged.
				support.RunBoth(func() {
					support.VisitTargetFilesOrdered(env, target, isTypeScriptLikeFile, func(file string, data []byte) func() {
						items := typeScriptUnresolvedMutationEvidence(env, file, data)
						return func() { addUnresolved(items) }
					})
				}, targetFindings)
				return findings
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"rust", "rs"},
			Run: func() []core.Finding {
				return support.ScanRustFiles(env, target, support.CacheSectionQualityRust, func(file string, data []byte) []core.Finding {
					return rustFindingsForFile(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"c++", "cpp", "cxx", "cc"},
			Run: func() []core.Finding {
				findings := support.ScanCPPFiles(env, target, support.CacheSectionQualityCPP, func(file string, data []byte) []core.Finding {
					return cppFindingsForFile(env, file, data)
				})
				if localPrecisionEnabled(env) && env.VisitTargetFiles != nil {
					support.VisitTargetFilesOrdered(env, target, func(file string) bool { return support.IsCPPPath(file, true) }, func(file string, data []byte) func() {
						items := cppUnresolvedMutationEvidence(env, file, data)
						return func() { addUnresolved(items) }
					})
				}
				return findings
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"java"},
			Run: func() []core.Finding {
				return env.ScanTargetFiles(target, support.CacheSectionQualityJava, isJavaFile, func(file string, data []byte) []core.Finding {
					return javaFindingsForFile(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"csharp", "c#", "cs", "dotnet"},
			Run: func() []core.Finding {
				return env.ScanTargetFiles(target, support.CacheSectionQualityCSharp, isCSharpFile, func(file string, data []byte) []core.Finding {
					return csharpFindingsForFile(env, file, data)
				})
			},
		},
		support.LanguageDispatch{
			Aliases: []string{"ruby", "rb"},
			Run: func() []core.Finding {
				return env.ScanTargetFiles(target, support.CacheSectionQualityRuby, isRubyFile, func(file string, data []byte) []core.Finding {
					return rubyFindingsForFile(env, file, data)
				})
			},
		},
	)
	return languageQualityScan{findings: findings, unresolved: unresolved}
}

// goQualityDependencies declares what the Go quality evaluator reads besides
// the scanned file: go.mod/go.work through the module resolver (import-path
// resolution for quality.dependency-direction), including the ones the
// listing does not carry (see goModuleResolverInputs), and, when local precision
// built a package-wide index, every sibling .go file in the same directory
// (package types and variables seen by the mutation resolver).
func goQualityDependencies(root string, index *goPackageIndex) support.FileScanDependencies {
	deps := support.FileScanDependencies{SharedFiles: isGoManifestPath, ExternalFiles: goModuleResolverInputs(root)}
	if index != nil {
		deps.PackageFiles = isGoSourcePath
	}
	return deps
}

// goModuleResolverInputs lists every manifest newGoModuleResolver and
// metadataForFile can read from disk for the scanned files, whether or not the
// walk lists it: the root go.work, the go.mod of each module it uses (which
// may lie outside the target), and a go.mod candidate in every directory from
// a scanned file up to the target root (the resolver takes the nearest one,
// even when config excludes it or .gitignore hides it).
func goModuleResolverInputs(root string) func([]string) []string {
	return func(listing []string) []string {
		inputs := []string{"go.work"}
		uses, _ := parseGoManifest(filepath.Join(root, "go.work"), true)
		for _, use := range uses {
			inputs = append(inputs, filepath.ToSlash(filepath.Join(use, "go.mod")))
		}
		seen := map[string]struct{}{}
		for _, rel := range listing {
			if !isGoSourcePath(rel) {
				continue
			}
			for dir := path.Dir(filepath.ToSlash(rel)); ; dir = path.Dir(dir) {
				if _, ok := seen[dir]; ok {
					break
				}
				seen[dir] = struct{}{}
				inputs = append(inputs, path.Join(dir, "go.mod"))
				if dir == "." || dir == "/" {
					break
				}
			}
		}
		return inputs
	}
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
