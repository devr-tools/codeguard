package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func runSecurity(t *testing.T, name, dir, language string) codeguard.Report {
	t.Helper()
	report, err := codeguard.Run(context.Background(), securityOnlyConfig(name, dir, language))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func TestSecurityDetectsMisconfiguration(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "server.py"), strings.Join([]string{
		"import flask",
		"app = flask.Flask(__name__)",
		"app.run(host='0.0.0.0', debug=True)",
		"HEADERS = {'Access-Control-Allow-Origin': '*'}",
		"",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "Dockerfile"), "FROM alpine\nUSER root\n")

	report := runSecurity(t, "a05", dir, "python")
	assertFindingRulePresent(t, report, "Security", "security.bind-all-interfaces")
	assertFindingRulePresent(t, report, "Security", "security.debug-enabled")
	assertFindingRulePresent(t, report, "Security", "security.cors-wildcard")
	assertFindingRulePresent(t, report, "Security", "security.dockerfile-root")
}

func TestSecurityDetectsWeakCrypto(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "crypto.py"), strings.Join([]string{
		"import hashlib",
		"from Crypto.Cipher import AES, DES",
		"digest = hashlib.md5(data).hexdigest()",
		"cipher = AES.new(key, AES.MODE_ECB)",
		"legacy = DES.new(key)",
		"",
	}, "\n"))

	report := runSecurity(t, "a02", dir, "python")
	assertFindingRulePresent(t, report, "Security", "security.weak-hash")
	assertFindingRulePresent(t, report, "Security", "security.weak-cipher")
}

func TestSecurityDetectsInsecureDeserialization(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "loader.py"), strings.Join([]string{
		"import pickle, yaml",
		"obj = pickle.loads(blob)",
		"cfg = yaml.load(text)",
		"",
	}, "\n"))

	report := runSecurity(t, "a08", dir, "python")
	assertFindingRulePresent(t, report, "Security", "security.insecure-deserialization")
}

func TestSecurityIgnoresSafeYAMLLoad(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "safe.py"), strings.Join([]string{
		"import yaml",
		"cfg = yaml.load(text, Loader=yaml.SafeLoader)",
		"safe = yaml.safe_load(text)",
		"",
	}, "\n"))

	report := runSecurity(t, "a08-safe", dir, "python")
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == "security.insecure-deserialization" {
				t.Fatalf("safe YAML loading should not be flagged: %s", finding.Message)
			}
		}
	}
}

func TestSecurityDetectsGoSSRF(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"net/http\"",
		"\t\"os\"",
		")",
		"",
		"func main() {",
		"\ttarget := os.Getenv(\"TARGET_URL\")",
		"\t_, _ = http.Get(target)",
		"}",
		"",
	}, "\n"))

	report := runSecurity(t, "a10-go", dir, "go")
	assertFindingRulePresent(t, report, "Security", "security.ssrf.go")
}

func TestSecurityDetectsPythonSSRF(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "fetch.py"), strings.Join([]string{
		"import os, requests",
		"url = os.environ['TARGET']",
		"resp = requests.get(url)",
		"",
	}, "\n"))

	report := runSecurity(t, "a10-py", dir, "python")
	assertFindingRulePresent(t, report, "Security", "security.ssrf.python")
}
