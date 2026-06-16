package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestPythonTaintInputToOsSystem(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), strings.Join([]string{
		"import os",
		"",
		"name = input('name? ')",
		"command = 'echo ' + name",
		"os.system(command)",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-py-system", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "fail")
	messages := taintMessages(t, report, "security.taint.python")
	assertChainMessage(t, messages, "input()", "os.system", "name -> command")
}

func TestPythonTaintRequestToCursorExecuteFString(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "views.py"), strings.Join([]string{
		"from flask import request",
		"",
		"def lookup(cursor):",
		"    user_id = request.args.get('id')",
		"    query = f\"SELECT * FROM users WHERE id = {user_id}\"",
		"    cursor.execute(query)",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-py-sql", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	messages := taintMessages(t, report, "security.taint.python")
	assertChainMessage(t, messages, "request.args", "cursor.execute", "user_id -> query")
}

func TestPythonTaintCrossFunctionFlows(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tool.py"), strings.Join([]string{
		"import os",
		"import subprocess",
		"import sys",
		"",
		"def read_target():",
		"    return sys.argv[1]",
		"",
		"def run_command(cmd):",
		"    subprocess.run(cmd, shell=True)",
		"",
		"def main():",
		"    target = read_target()",
		"    run_command(target)",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-py-cross", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	messages := taintMessages(t, report, "security.taint.python")
	assertChainMessage(t, messages, "sys.argv", "read_target()", "subprocess.run")
	assertChainMessage(t, messages, "run_command()")
}

func TestPythonTaintEvalOfEnviron(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cfg.py"), strings.Join([]string{
		"import os",
		"",
		"expr = os.environ.get('RULE')",
		"eval(expr)",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-py-eval", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	messages := taintMessages(t, report, "security.taint.python")
	assertChainMessage(t, messages, "os.environ", "eval", "expr")
}

func TestPythonTaintSanitizedFlowsDoNotFlag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "safe.py"), strings.Join([]string{
		"import os",
		"import shlex",
		"import subprocess",
		"",
		"name = input('name? ')",
		"os.system('echo ' + shlex.quote(name))",
		"",
		"count = int(input('count? '))",
		"os.system(f'head -n {count} log.txt')",
		"",
		"def lookup(cursor, user_id):",
		"    cursor.execute('SELECT * FROM users WHERE id = %s', (user_id,))",
		"",
		"subprocess.run(['echo', name])",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-py-safe", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if messages := taintMessages(t, report, "security.taint.python"); len(messages) != 0 {
		t.Fatalf("sanitized flows must not flag, got %v", messages)
	}
}

func TestPythonTaintCommentsAndStringsDoNotFlag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "docs.py"), strings.Join([]string{
		"DOC = \"\"\"",
		"os.system(input())",
		"\"\"\"",
		"# os.system(input('never'))",
		"text = \"eval(input())\"",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-py-docs", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if messages := taintMessages(t, report, "security.taint.python"); len(messages) != 0 {
		t.Fatalf("strings and comments must not flag, got %v", messages)
	}
}

func TestPythonTaintToggleDisables(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), "import os\nos.system(input())\n")

	cfg := securityOnlyConfig("taint-py-toggle", dir, "python")
	disabled := false
	cfg.Checks.SecurityRules.TaintPython = &disabled

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if messages := taintMessages(t, report, "security.taint.python"); len(messages) != 0 {
		t.Fatalf("taint_python=false must disable the rule, got %v", messages)
	}
}
