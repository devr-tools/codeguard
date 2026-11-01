package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var securityExamples = map[string][]core.RuleExample{
	"security.taint.go": {goExample("main.go", `package main

import (
	"os"
	"os/exec"
)

func main() {
	userCmd := os.Getenv("USER_CMD")
	_ = exec.Command("sh", "-c", userCmd).Run()
}
`, `package main

import (
	"os"
	"os/exec"
	"strconv"
)

func main() {
	lines, err := strconv.Atoi(os.Getenv("LINES"))
	if err != nil {
		return
	}
	_ = exec.Command("tail", "-n", strconv.Itoa(lines), "app.log").Run()
}
`)},
	"security.ssrf.go": {goExample("fetch.go", `package main

import (
	"net/http"
	"os"
)

func main() {
	target := os.Getenv("TARGET_URL")
	_, _ = http.Get(target)
}
`, `package main

import "net/http"

func main() {
	_, _ = http.Get("https://status.example.com/healthz")
}
`)},
	"security.taint.python": {pythonExample("app.py", `import os

name = input("name? ")
os.system("echo " + name)
`, `import os
import shlex

name = input("name? ")
os.system("echo " + shlex.quote(name))
`)},
	"security.ssrf.python": {pythonExample("fetch.py", `import os

import requests

url = os.environ["TARGET"]
response = requests.get(url, timeout=5)
`, `import requests

response = requests.get("https://api.example.com/health", timeout=5)
`)},
	"security.taint.cpp": {cppExample("main.cpp", `#include <cstdlib>

int main(int argc, char** argv) {
  std::system(argv[1]);
}
`, `#include <cstdlib>
#include <string>

int main(int argc, char** argv) {
  int lines = std::stoi(argv[1]);
  std::system(("tail -n " + std::to_string(lines) + " app.log").c_str());
}
`)},
	"security.ssrf.cpp": {cppExample("handler.cpp", `void handler(const drogon::HttpRequest* request) {
  auto target = request->getParameter("url");
  cpr::Get(cpr::Url{target});
}
`, `void handler(const drogon::HttpRequest* request) {
  auto target = allowlisted_url(request->getParameter("url"));
  cpr::Get(cpr::Url{target});
}
`)},
	"security.typescript.taint-flow": {typescriptExample("src/handler.ts", `import type { Request } from "express";
import { exec } from "child_process";

export function handler(req: Request): void {
  exec(req.query.cmd as string);
}
`, `import type { Request } from "express";
import { exec } from "child_process";

declare function shellQuote(value: string): string;

export function handler(req: Request): void {
  exec(shellQuote(req.query.cmd as string));
}
`)},
	"security.javascript.taint-flow": {javascriptExample("src/handler.js", `const { exec } = require("child_process");

function handler(req, res) {
  exec(req.query.cmd);
  res.end();
}

module.exports = { handler };
`, `const { exec } = require("child_process");
const shellQuote = require("shell-quote").quote;

function handler(req, res) {
  exec(shellQuote([req.query.cmd]));
  res.end();
}

module.exports = { handler };
`)},
}
