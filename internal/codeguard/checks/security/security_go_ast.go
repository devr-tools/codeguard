package security

import (
	"go/ast"
	"go/token"
	"strconv"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// goSecurityFindings runs the AST-based Go pass for the base security checks
// (security.insecure-tls and security.shell-execution), so imports, comments,
// and string literals cannot trigger findings. It reports ok=false when the
// file does not parse, in which case the caller falls back to the masked line
// scan in security_common.go.
func goSecurityFindings(env support.Context, file string, data []byte) (findings []core.Finding, ok bool) {
	fset, parsed, err := support.ParseGoSource(env, file, data)
	if err != nil || parsed == nil {
		return nil, false
	}
	inspector := &goSecurityInspector{env: env, file: file, fset: fset}
	inspector.bindImports(parsed)
	ast.Inspect(parsed, inspector.visit)
	return inspector.findings, true
}

// goSecurityInspector walks one parsed Go file and collects findings for the
// base security checks.
type goSecurityInspector struct {
	env        support.Context
	file       string
	fset       *token.FileSet
	execPkg    string
	syscallPkg string
	findings   []core.Finding
}

// bindImports records the local package names for os/exec and syscall,
// honouring aliases. Blank and dot imports yield no name: blank imports have
// no call sites, and dot-imported call sites are not resolved (a known
// limitation of this syntactic pass).
func (in *goSecurityInspector) bindImports(parsed *ast.File) {
	for _, spec := range parsed.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		switch path {
		case "os/exec":
			in.execPkg = goImportLocalName(spec, "exec")
		case "syscall":
			in.syscallPkg = goImportLocalName(spec, "syscall")
		}
	}
}

func goImportLocalName(spec *ast.ImportSpec, defaultName string) string {
	if spec.Name == nil {
		return defaultName
	}
	if name := spec.Name.Name; name != "_" && name != "." {
		return name
	}
	return ""
}

func (in *goSecurityInspector) visit(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.KeyValueExpr:
		// Composite literal field, e.g. tls.Config{InsecureSkipVerify: true}.
		if isIdentNamed(n.Key, "InsecureSkipVerify") && isTrueLiteral(n.Value) {
			in.addInsecureTLS(n.Key.Pos())
		}
	case *ast.AssignStmt:
		in.visitAssign(n)
	case *ast.CallExpr:
		in.visitCall(n)
	}
	return true
}

// visitAssign flags cfg.InsecureSkipVerify = true assignments. Values that are
// not the literal `true` (variables, function results) are intentionally not
// flagged: without type information this pass cannot tell whether they are
// insecure, and a spurious fail is worse than a miss here.
func (in *goSecurityInspector) visitAssign(assign *ast.AssignStmt) {
	if len(assign.Lhs) != len(assign.Rhs) {
		return
	}
	for i, lhs := range assign.Lhs {
		selector, isSelector := lhs.(*ast.SelectorExpr)
		if !isSelector || selector.Sel.Name != "InsecureSkipVerify" {
			continue
		}
		if isTrueLiteral(assign.Rhs[i]) {
			in.addInsecureTLS(selector.Sel.Pos())
		}
	}
}

// visitCall flags call sites of exec.Command, exec.CommandContext, and
// syscall.Exec, using the import-bound package names so aliased imports are
// followed and files that never import the packages cannot fire.
func (in *goSecurityInspector) visitCall(call *ast.CallExpr) {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return
	}
	pkg, isIdent := selector.X.(*ast.Ident)
	if !isIdent {
		return
	}
	execCall := in.execPkg != "" && pkg.Name == in.execPkg &&
		(selector.Sel.Name == "Command" || selector.Sel.Name == "CommandContext")
	syscallCall := in.syscallPkg != "" && pkg.Name == in.syscallPkg && selector.Sel.Name == "Exec"
	if execCall || syscallCall {
		in.addShellExecution(call.Pos())
	}
}

func (in *goSecurityInspector) addInsecureTLS(pos token.Pos) {
	in.findings = append(in.findings, in.env.NewFinding(support.FindingInput{RuleID: "security.insecure-tls", Level: "fail", Path: in.file, Line: in.fset.Position(pos).Line, Column: 1, Message: "InsecureSkipVerify is enabled"}))
}

func (in *goSecurityInspector) addShellExecution(pos token.Pos) {
	in.findings = append(in.findings, in.env.NewFinding(support.FindingInput{RuleID: "security.shell-execution", Level: "warn", Path: in.file, Line: in.fset.Position(pos).Line, Column: 1, Message: "shell execution primitive should be reviewed"}))
}

func isIdentNamed(expr ast.Expr, name string) bool {
	ident, isIdent := expr.(*ast.Ident)
	return isIdent && ident.Name == name
}

// isTrueLiteral reports whether expr is the predeclared literal `true`.
func isTrueLiteral(expr ast.Expr) bool {
	return isIdentNamed(expr, "true")
}
