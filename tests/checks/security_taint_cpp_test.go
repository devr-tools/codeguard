package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestCPPTaintEnvironmentAndArgvReachProcessSinks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.cpp"), strings.Join([]string{
		"#include <cstdlib>",
		"#include <string>",
		"",
		"std::string command_from_env() {",
		"  return std::getenv(\"USER_COMMAND\");",
		"}",
		"",
		"void launch(const std::string& command) {",
		"  std::system(command.c_str());",
		"}",
		"",
		"int main(int argc, char** argv) {",
		"  auto command = command_from_env();",
		"  launch(command);",
		"  auto alternate = argv[1];",
		"  popen(alternate, \"r\");",
		"}",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-cpp-process", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	messages := taintMessages(t, report, "security.taint.cpp")
	assertChainMessage(t, messages, "std::getenv()", "std::system", "command_from_env()", "launch()")
	assertChainMessage(t, messages, "main parameter argv", "popen", "alternate")
	assertFindingConfidence(t, report, "Security", "security.taint.cpp", "high")
}

func TestCPPSSRFRecognizesLibcurlCPRAndBoostResolver(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "fetch.cpp"), strings.Join([]string{
		"#include <cstdlib>",
		"",
		"void fetch(CURL* curl, const std::string& target) {",
		"  curl_easy_setopt(curl, CURLOPT_URL, target.c_str());",
		"}",
		"",
		"void handler(const crow::request& request, CURL* curl) {",
		"  auto target = request.url_params.get(\"url\");",
		"  fetch(curl, target);",
		"  cpr::Get(cpr::Url{target});",
		"  tcp_resolver.resolve(target, \"443\");",
		"}",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-cpp-ssrf", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	messages := taintMessages(t, report, "security.ssrf.cpp")
	assertChainMessage(t, messages, "request request data", "curl_easy_setopt[CURLOPT_URL]", "target", "fetch()")
	assertChainMessage(t, messages, "request request data", "cpr::Get", "target")
	assertChainMessage(t, messages, "request request data", "tcp_resolver.resolve", "target")
}

func TestCPPSSRFRecognizesPointerRequestParameters(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.cpp"), strings.Join([]string{
		"void handler(const drogon::HttpRequest* request) {",
		"  auto target = request->getParameter(\"url\");",
		"  cpr::Get(cpr::Url{target});",
		"}",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-cpp-request-pointer", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertChainMessage(t, taintMessages(t, report, "security.ssrf.cpp"), "request request data", "cpr::Get", "target")
}

func TestCPPTaintSourcesFromStandardInput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "stdin.cpp"), strings.Join([]string{
		"void run() {",
		"  std::string first;",
		"  std::cin >> first;",
		"  std::system(first.c_str());",
		"  std::string second;",
		"  std::getline(std::cin, second);",
		"  popen(second.c_str(), \"r\");",
		"}",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-cpp-stdin", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	messages := taintMessages(t, report, "security.taint.cpp")
	assertChainMessage(t, messages, "std::cin", "std::system", "first")
	assertChainMessage(t, messages, "std::getline(std::cin)", "popen", "second")
}

func TestCPPTaintIgnoresConstantsSanitizersCommentsAndStrings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "safe.cpp"), strings.Join([]string{
		"void safe(CURL* curl) {",
		"  std::system(\"date\");",
		"  curl_easy_setopt(curl, CURLOPT_URL, \"https://api.example.com\");",
		"  auto numeric = std::stoi(std::getenv(\"ACTION\"));",
		"  std::system(std::to_string(numeric).c_str());",
		"  auto target = allowlisted_url(std::getenv(\"TARGET_URL\"));",
		"  cpr::Get(cpr::Url{target});",
		"  // auto bad = std::getenv(\"BAD\"); std::system(bad);",
		"  const char* sample = R\"(curl_easy_setopt(curl, CURLOPT_URL, argv[1]);)\";",
		"}",
		"",
	}, "\n"))

	report, err := codeguard.Run(context.Background(), securityOnlyConfig("taint-cpp-safe", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if messages := taintMessages(t, report, "security.taint.cpp"); len(messages) != 0 {
		t.Fatalf("expected no C++ process taint findings, got %v", messages)
	}
	if messages := taintMessages(t, report, "security.ssrf.cpp"); len(messages) != 0 {
		t.Fatalf("expected no C++ SSRF findings, got %v", messages)
	}
}

func TestCPPTaintCanBeDisabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.cpp"), "int main(int argc, char** argv) { std::system(argv[1]); }\n")

	disabled := false
	cfg := securityOnlyConfig("taint-cpp-toggle", dir, "cpp")
	cfg.Checks.SecurityRules.TaintCPP = &disabled
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if messages := taintMessages(t, report, "security.taint.cpp"); len(messages) != 0 {
		t.Fatalf("taint_cpp=false must disable the rule, got %v", messages)
	}
}
