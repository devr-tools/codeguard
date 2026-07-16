package performance

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// goAllocLoopScan carries the per-function context of the alloc-in-loop
// inspection so the assignment classifier stays within the parameter budget.
type goAllocLoopScan struct {
	env            support.Context
	file           string
	fset           *token.FileSet
	growable       map[string]struct{}
	detectPrealloc bool
}

// goAllocInLoopFindings flags allocation-heavy loop bodies. String growth by
// concatenation (including fmt.Sprintf accumulation) is reported whenever
// detect_alloc_in_loop is on because the cost is quadratic. Append without
// preallocation is gated separately behind detect_prealloc_in_loop (off by
// default) because it is a micro-optimization that idiomatic accumulation
// loops legitimately skip.
func goAllocInLoopFindings(env support.Context, file string, fset *token.FileSet, parsed *ast.File) []core.Finding {
	findings := make([]core.Finding, 0)
	detectPrealloc := preallocToggleEnabled(env.Config.Checks.PerformanceRules.DetectPreallocInLoop)
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		scan := goAllocLoopScan{
			env:            env,
			file:           file,
			fset:           fset,
			growable:       goGrowableSliceNames(fn.Body),
			detectPrealloc: detectPrealloc,
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			body := goLoopBody(node)
			if body == nil {
				return true
			}
			knowable := goLoopBoundKnowable(node)
			ast.Inspect(body, func(inner ast.Node) bool {
				assign, ok := inner.(*ast.AssignStmt)
				if !ok {
					return true
				}
				findings = append(findings, scan.assignFindings(assign, knowable)...)
				return true
			})
			return true
		})
	}
	return dedupeFindingsByLine(findings)
}

// preallocToggleEnabled treats a nil toggle as disabled because the prealloc
// branch must stay opt-in, unlike the other quality toggles.
func preallocToggleEnabled(value *bool) bool {
	return value != nil && *value
}

func (scan goAllocLoopScan) assignFindings(assign *ast.AssignStmt, knowableBound bool) []core.Finding {
	if message := goStringGrowthMessage(assign); message != "" {
		return []core.Finding{scan.finding(assign, message)}
	}
	if !scan.detectPrealloc || !knowableBound {
		return nil
	}
	name, ok := goSelfAppendTarget(assign)
	if !ok {
		return nil
	}
	if _, candidate := scan.growable[name]; !candidate {
		return nil
	}
	message := fmt.Sprintf("append to slice %q inside a loop with a knowable bound; preallocate capacity with make before the loop", name)
	return []core.Finding{scan.finding(assign, message)}
}

func (scan goAllocLoopScan) finding(assign *ast.AssignStmt, message string) core.Finding {
	pos := scan.fset.Position(assign.Pos())
	return warnFinding(scan.env, "performance.go.alloc-in-loop", scan.file, pos.Line, pos.Column, message)
}

func goStringGrowthMessage(assign *ast.AssignStmt) string {
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return ""
	}
	target, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return ""
	}
	rhs := assign.Rhs[0]
	switch assign.Tok {
	case token.ADD_ASSIGN:
	case token.ASSIGN:
		binary, isBinary := rhs.(*ast.BinaryExpr)
		if !isBinary || binary.Op != token.ADD || !goExprMentionsIdent(binary, target.Name) {
			return ""
		}
	default:
		return ""
	}
	if !goExprLooksLikeString(rhs) {
		return ""
	}
	if goExprUsesSprintf(rhs) {
		return fmt.Sprintf("string %q accumulates fmt.Sprintf output inside a loop; use strings.Builder or fmt.Fprintf on a builder", target.Name)
	}
	return fmt.Sprintf("string %q grows by concatenation inside a loop; use strings.Builder", target.Name)
}

func goSelfAppendTarget(assign *ast.AssignStmt) (string, bool) {
	if assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return "", false
	}
	target, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return "", false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || len(call.Args) < 2 {
		return "", false
	}
	fun, ok := call.Fun.(*ast.Ident)
	if !ok || fun.Name != "append" {
		return "", false
	}
	first, ok := call.Args[0].(*ast.Ident)
	if !ok || first.Name != target.Name {
		return "", false
	}
	return target.Name, true
}

func dedupeFindingsByLine(findings []core.Finding) []core.Finding {
	seen := make(map[string]struct{}, len(findings))
	out := make([]core.Finding, 0, len(findings))
	for _, finding := range findings {
		key := fmt.Sprintf("%s:%d:%s", finding.RuleID, finding.Line, finding.Message)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, finding)
	}
	return out
}
