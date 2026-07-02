package checks_test

import (
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

type a09Case struct {
	name     string
	file     string
	language string
	source   string
	want     bool
}

func assertA09Case(t *testing.T, ruleID string, tc a09Case) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, tc.file), tc.source)
		report := runSecurity(t, "a09-"+tc.name, dir, tc.language)
		if tc.want {
			assertFindingRulePresent(t, report, "Security", ruleID)
			return
		}
		assertFindingRuleAbsent(t, report, "Security", ruleID)
	})
}

func TestSecurityLogSecretExposure(t *testing.T) {
	cases := []a09Case{
		// True positives.
		{"go-identifier-argument", "main.go", "go",
			"package main\n\nimport \"log\"\n\nfunc login(password string) {\n\tlog.Printf(\"login attempt with %s\", password)\n}\n", true},
		{"go-authorization-concat", "main.go", "go",
			"package main\n\nimport \"log\"\n\nfunc trace(tok string) {\n\tlog.Println(\"Authorization: Bearer \" + tok)\n}\n", true},
		{"go-slog-structured-key", "main.go", "go",
			"package main\n\nimport \"log/slog\"\n\nfunc audit(value string) {\n\tslog.Info(\"login\", \"api_key\", value)\n}\n", true},
		{"go-format-directive", "main.go", "go",
			"package main\n\nimport \"log\"\n\nfunc trace(t string) {\n\tlog.Printf(\"session token=%s\", t)\n}\n", true},
		{"python-identifier-argument", "app.py", "python",
			"import logging\nlogger = logging.getLogger(__name__)\n\ndef login(password):\n    logger.info(\"login for %s\", password)\n", true},
		{"python-fstring-interpolation", "app.py", "python",
			"def trace(token):\n    print(f\"issued token={token}\")\n", true},
		{"typescript-template-interpolation", "src/auth.ts", "typescript",
			"export function trace(token: string) {\n  console.log(`Bearer ${token}`);\n}\n", true},
		{"javascript-camelcase-identifier", "src/auth.js", "javascript",
			"function audit(apiKey) {\n  logger.error(\"auth failed\", apiKey);\n}\n", true},
		// Adversarial negatives.
		{"go-token-in-comment", "main.go", "go",
			"package main\n\nimport \"log\"\n\nfunc refresh() {\n\t// refresh the token cache before retrying\n\tlog.Println(\"cache refreshed\")\n}\n", false},
		{"go-plain-format-literal", "main.go", "go",
			"package main\n\nimport \"log\"\n\nfunc report(n int) {\n\tlog.Printf(\"token count: %d\", n)\n}\n", false},
		{"go-tokenizer-identifier", "main.go", "go",
			"package main\n\nimport \"log\"\n\nfunc report(tokenizer parser) {\n\tlog.Printf(\"parsed %d nodes\", tokenizer.Count)\n}\n", false},
		{"python-token-in-comment", "app.py", "python",
			"# rotate the password file weekly\nprint(\"rotation scheduled\")\n", false},
		{"python-plain-format-literal", "app.py", "python",
			"import logging\nlogging.info(\"token count: %d\", n)\n", false},
		{"typescript-plain-literal", "src/report.ts", "typescript",
			"export function report(n: number) {\n  console.log(\"token count:\", n);\n}\n", false},
		{"javascript-secret-outside-call", "src/report.js", "javascript",
			"const password = load();\nconsole.log(\"loaded configuration\");\n", false},
	}
	for _, tc := range cases {
		assertA09Case(t, "security.log-secret-exposure", tc)
	}
}

func TestSecurityUnsanitizedErrorResponse(t *testing.T) {
	cases := []a09Case{
		// True positives.
		{"go-http-error-raw", "handler.go", "go",
			"package main\n\nimport \"net/http\"\n\nfunc handle(w http.ResponseWriter, r *http.Request) {\n\terr := do()\n\tif err != nil {\n\t\thttp.Error(w, err.Error(), http.StatusInternalServerError)\n\t}\n}\n", true},
		{"go-fprintf-raw-err", "handler.go", "go",
			"package main\n\nimport (\n\t\"fmt\"\n\t\"net/http\"\n)\n\nfunc handle(w http.ResponseWriter, r *http.Request) {\n\tif err := do(); err != nil {\n\t\tfmt.Fprintf(w, \"failed: %v\", err)\n\t}\n}\n", true},
		{"typescript-res-json-err", "src/handler.ts", "typescript",
			"app.get(\"/x\", (req, res) => {\n  run().catch((err) => {\n    res.json(err);\n  });\n});\n", true},
		{"javascript-status-send-stack", "src/handler.js", "javascript",
			"app.use((err, req, res, next) => {\n  res.status(500).send(err.stack || err.message);\n});\n", true},
		{"python-return-str-exception", "views.py", "python",
			"def handler(request):\n    try:\n        work()\n    except ValueError as e:\n        return str(e)\n", true},
		{"python-httpresponse-str-exception", "views.py", "python",
			"from django.http import HttpResponse\n\ndef handler(request):\n    try:\n        work()\n    except Exception as exc:\n        return HttpResponse(str(exc))\n", true},
		// Adversarial negatives.
		{"go-err-logged-not-returned", "handler.go", "go",
			"package main\n\nimport (\n\t\"log\"\n\t\"net/http\"\n)\n\nfunc handle(w http.ResponseWriter, r *http.Request) {\n\tif err := do(); err != nil {\n\t\tlog.Printf(\"handle failed: %v\", err)\n\t\thttp.Error(w, \"internal server error\", http.StatusInternalServerError)\n\t}\n}\n", false},
		{"go-fprintf-non-writer", "report.go", "go",
			"package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc report(err error) {\n\tfmt.Fprintf(os.Stderr, \"failed: %v\", err)\n}\n", false},
		{"typescript-err-logged-generic-response", "src/handler.ts", "typescript",
			"app.use((err, req, res, next) => {\n  logger.error(err);\n  res.status(500).send(\"internal server error\");\n});\n", false},
		{"python-str-outside-except", "views.py", "python",
			"def render(e):\n    return str(e)\n", false},
		{"python-generic-response-in-except", "views.py", "python",
			"from django.http import HttpResponse\n\ndef handler(request):\n    try:\n        work()\n    except ValueError as e:\n        logger.error(str(e))\n        return HttpResponse(\"internal server error\")\n", false},
	}
	for _, tc := range cases {
		assertA09Case(t, "security.unsanitized-error-response", tc)
	}
}

func TestPythonTaintRulesAreLanguageAgnostic(t *testing.T) {
	rules := map[string]codeguard.RuleMetadata{}
	for _, rule := range codeguard.Rules() {
		rules[rule.ID] = rule
	}
	for _, id := range []string{"security.taint.python", "security.ssrf.python"} {
		rule, ok := rules[id]
		if !ok {
			t.Fatalf("rule %s not found in catalog", id)
		}
		if rule.ExecutionModel != codeguard.RuleExecutionModelLanguageAgnostic {
			t.Errorf("%s execution model = %q, want %q", id, rule.ExecutionModel, codeguard.RuleExecutionModelLanguageAgnostic)
		}
		if len(rule.LanguageCoverage.Languages) != 1 || rule.LanguageCoverage.Languages[0] != codeguard.RuleLanguagePython {
			t.Errorf("%s language coverage = %v, want [python]", id, rule.LanguageCoverage.Languages)
		}
	}
}
