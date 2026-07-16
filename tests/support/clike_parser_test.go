package support_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

const trickyTypeScript = "import fs from 'fs';\n" +
	"import { exec as run, spawn } from 'child_process';\n" +
	"const path = require('path');\n" +
	"\n" +
	"// function commented(x) { return x; }\n" +
	"const snippet = `function templated(y) {\n" +
	"  return y;\n" +
	"}`;\n" +
	"\n" +
	"export async function handler(\n" +
	"  request: Request,\n" +
	"  retries: number = 3,\n" +
	"): Promise<string> {\n" +
	"  const command = `ls ${request.url}`;\n" +
	"  let label = 'literal { brace';\n" +
	"  function helper(input: string) {\n" +
	"    return input.trim();\n" +
	"  }\n" +
	"  return helper(command);\n" +
	"}\n" +
	"\n" +
	"const arrow = (a: number, b: number): number => a + b;\n"

func TestParseTypeScriptStructure(t *testing.T) {
	file := support.ParseCLike(trickyTypeScript, support.CLikeTypeScript)

	if file.FunctionByName("commented") != nil {
		t.Fatal("function inside comment must not parse")
	}
	if file.FunctionByName("templated") != nil {
		t.Fatal("function inside template literal must not parse")
	}
	handler := file.FunctionByName("handler")
	if handler == nil {
		t.Fatalf("handler not found; functions: %v", functionNames(file))
	}
	if len(handler.Params) != 2 || handler.Params[0].Name != "request" {
		t.Fatalf("handler params = %+v", handler.Params)
	}
	if handler.StartLine != 10 || handler.EndLine != 20 {
		t.Fatalf("handler span = %d..%d, want 10..20", handler.StartLine, handler.EndLine)
	}
	if len(handler.Nested) != 1 || handler.Nested[0].Name != "helper" {
		t.Fatalf("nested = %+v", handler.Nested)
	}
	if file.FunctionByName("arrow") == nil {
		t.Fatalf("arrow function not found; functions: %v", functionNames(file))
	}
}

func TestParseTypeScriptScopeAndImports(t *testing.T) {
	file := support.ParseCLike(trickyTypeScript, support.CLikeTypeScript)
	handler := file.FunctionByName("handler")

	if kind, ok := handler.Lookup("command"); !ok || kind != support.SymbolLocal {
		t.Fatalf("command = (%v,%v), want local", kind, ok)
	}
	if kind, ok := handler.Lookup("request"); !ok || kind != support.SymbolParam {
		t.Fatalf("request = (%v,%v), want param", kind, ok)
	}

	var command *support.ParsedAssignment
	for idx := range handler.Assignments {
		if handler.Assignments[idx].Name == "command" {
			command = &handler.Assignments[idx]
		}
	}
	if command == nil {
		t.Fatalf("command assignment missing: %+v", handler.Assignments)
	}
	if !strings.Contains(command.Expr, "${request.url}") {
		t.Fatalf("template interpolation lost: %q", command.Expr)
	}
	if strings.Contains(command.Expr, "ls") {
		t.Fatalf("template text must be masked: %q", command.Expr)
	}

	if !hasImport(file.Imports, "child_process", "run") {
		t.Fatalf("aliased named import missing: %+v", file.Imports)
	}
	if !hasImport(file.Imports, "fs", "fs") || !hasImport(file.Imports, "path", "path") {
		t.Fatalf("default/require imports missing: %+v", file.Imports)
	}
}

const trickyRust = `use std::process::Command;
use std::io::{self, Read as ReadExt};

// fn commented(x: i32) -> i32 { x }

pub fn shell<'a>(
    input: &'a str,
    count: usize,
) -> String {
    let pattern = r#"fn raw_inner() { panic!("}") }"#;
    let mut owned = String::from(input);
    fn nested(v: &str) -> usize { v.len() }
    owned.push('}');
    format!("{} {}", owned, count)
}
`

func TestParseRustStructure(t *testing.T) {
	file := support.ParseCLike(trickyRust, support.CLikeRust)

	if file.FunctionByName("commented") != nil {
		t.Fatal("function inside comment must not parse")
	}
	if file.FunctionByName("raw_inner") != nil {
		t.Fatal("function inside raw string must not parse")
	}
	shell := file.FunctionByName("shell")
	if shell == nil {
		t.Fatalf("shell not found; functions: %v", functionNames(file))
	}
	if len(shell.Params) != 2 || shell.Params[0].Name != "input" {
		t.Fatalf("shell params = %+v", shell.Params)
	}
	if shell.EndLine != 15 {
		t.Fatalf("shell end line = %d, want 15 (brace inside char literal must not close body)", shell.EndLine)
	}
	if len(shell.Nested) != 1 || shell.Nested[0].Name != "nested" {
		t.Fatalf("nested = %+v", shell.Nested)
	}
	if kind, ok := shell.Lookup("owned"); !ok || kind != support.SymbolLocal {
		t.Fatalf("owned = (%v,%v), want local", kind, ok)
	}
	if !hasImport(file.Imports, "std::process::Command", "Command") {
		t.Fatalf("rust use missing: %+v", file.Imports)
	}
	if !hasImport(file.Imports, "std::io::Read", "ReadExt") {
		t.Fatalf("grouped aliased use missing: %+v", file.Imports)
	}
}

const trickyJava = `package demo;

import java.sql.Statement;
import static java.util.Objects.requireNonNull;

public class Repo {
    // public void commented(int x) { }
    private static final String QUERY = "SELECT } FROM t";

    public String find(
            Statement statement,
            String userId) throws Exception {
        String query = "SELECT * FROM users WHERE id = " + userId;
        String block = """
                void textBlockFake() { }
                """;
        return query;
    }
}
`

func TestParseJavaStructure(t *testing.T) {
	file := support.ParseCLike(trickyJava, support.CLikeJava)

	if file.FunctionByName("commented") != nil {
		t.Fatal("method inside comment must not parse")
	}
	if file.FunctionByName("textBlockFake") != nil {
		t.Fatal("method inside text block must not parse")
	}
	find := file.FunctionByName("find")
	if find == nil {
		t.Fatalf("find not found; functions: %v", functionNames(file))
	}
	if len(find.Params) != 2 || find.Params[1].Name != "userId" || find.Params[0].Type != "Statement" {
		t.Fatalf("find params = %+v", find.Params)
	}
	if find.StartLine != 10 || find.EndLine != 18 {
		t.Fatalf("find span = %d..%d, want 10..18", find.StartLine, find.EndLine)
	}
	if kind, ok := find.Lookup("query"); !ok || kind != support.SymbolLocal {
		t.Fatalf("query = (%v,%v), want local", kind, ok)
	}
	if !hasImport(file.Imports, "java.sql.Statement", "Statement") {
		t.Fatalf("java import missing: %+v", file.Imports)
	}
}

const trickyCPP = `#include <regex>
#include "widget.hpp"

// int commented(int x) { return x; }
std::string Demo::render(const std::vector<std::string>& rows, int count) {
    const char* raw = R"(int fake_inner() { return 1; })";
    std::string out = "";
    auto nested = [](const std::string& row) { return row.size(); };
    out += rows.front();
    return out;
}
`

func TestParseCPPStructure(t *testing.T) {
	file := support.ParseCLike(trickyCPP, support.CLikeCPP)

	if file.FunctionByName("commented") != nil {
		t.Fatal("function inside comment must not parse")
	}
	if file.FunctionByName("fake_inner") != nil {
		t.Fatal("function inside raw string must not parse")
	}
	render := file.FunctionByName("Demo::render")
	if render == nil {
		t.Fatalf("render not found; functions: %v", functionNames(file))
	}
	if len(render.Params) != 2 || render.Params[0].Name != "rows" || render.Params[1].Name != "count" {
		t.Fatalf("render params = %+v", render.Params)
	}
	if kind, ok := render.Lookup("out"); !ok || kind != support.SymbolLocal {
		t.Fatalf("out = (%v,%v), want local", kind, ok)
	}
	if !hasImport(file.Imports, "regex", "regex") || !hasImport(file.Imports, "widget.hpp", "widget.hpp") {
		t.Fatalf("cpp includes missing: %+v", file.Imports)
	}
}

func functionNames(file *support.ParsedFile) []string {
	allFns := file.AllFunctions()
	names := make([]string, 0, len(allFns))
	for _, fn := range allFns {
		names = append(names, fn.Name)
	}
	return names
}
