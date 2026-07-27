package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func observabilityConfig(name string, dir string, language string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SupplyChain = false
	on := true
	off := false
	cfg.Checks.Reliability = &off
	cfg.Checks.Data = &off
	cfg.Checks.Change = &off
	cfg.Checks.Observability = &on
	cfg.Checks.Operations = &off
	cfg.Checks.Context = &off
	cfg.Cache.Enabled = &off
	return cfg
}

func TestObservabilityGoDetectsLoggingAndMetricsRisks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.go"), `package sample

import "fmt"

func CheckoutHandler(userID string, token string, err error) {
	fmt.Println("checkout failed", err)
	logger.Error(err)
	logger.Info("token", token)
	requests.WithLabelValues(userID).Inc()
}
`)

	report, err := codeguard.Run(context.Background(), observabilityConfig("observability-go", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Observability", "observability.unstructured-log")
	assertFindingRulePresent(t, report, "Observability", "observability.error-without-context")
	assertFindingRulePresent(t, report, "Observability", "observability.sensitive-log-data")
	assertFindingRulePresent(t, report, "Observability", "observability.high-cardinality-label")
}

func TestObservabilityDetectsCriticalPathLogIgnoreAndShallowHealth(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "api.py"), `import requests

def payment_handler(err):
    logging.error(err)
    return None

def healthz():
    db = requests.get("http://db")
    return "ok"
`)

	report, err := codeguard.Run(context.Background(), observabilityConfig("observability-python", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Observability", "observability.log-and-ignore")
	assertFindingRulePresent(t, report, "Observability", "observability.shallow-health-check")
	assertFindingRulePresent(t, report, "Observability", "observability.critical-path-uninstrumented")
}

func TestObservabilityTypeScriptAndCPPDetectors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "consumer.ts"), `export function orderConsumer(event: Event, request_id: string) {
  console.log("received", event)
  logger.error("failed")
  latency.labels({ request_id }).observe(1)
}
`)
	writeFile(t, filepath.Join(dir, "worker.cpp"), `#include <cstdio>

void auth_worker(const char* password) {
  printf("password=%s", password);
}
`)

	tsReport, err := codeguard.Run(context.Background(), observabilityConfig("observability-ts", dir, "typescript"))
	if err != nil {
		t.Fatalf("run ts: %v", err)
	}
	assertFindingRulePresent(t, tsReport, "Observability", "observability.unstructured-log")
	assertFindingRulePresent(t, tsReport, "Observability", "observability.high-cardinality-label")

	cppReport, err := codeguard.Run(context.Background(), observabilityConfig("observability-cpp", dir, "c++"))
	if err != nil {
		t.Fatalf("run cpp: %v", err)
	}
	assertFindingRulePresent(t, cppReport, "Observability", "observability.sensitive-log-data")
}

func TestObservabilityAllowsStructuredInstrumentedCriticalPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.go"), `package sample

func CheckoutHandler(ctx context.Context, orderID string, err error) {
	span := tracer.Start(ctx, "checkout")
	defer span.End()
	logger.Error("checkout failed", "operation", "checkout", "order", orderID, "err", err)
	metrics.Counter("checkout_failed").Inc()
}
`)

	report, err := codeguard.Run(context.Background(), observabilityConfig("observability-safe", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Observability", "observability.error-without-context")
	assertFindingRuleAbsent(t, report, "Observability", "observability.critical-path-uninstrumented")
}

func TestObservabilityDetectorMatrixAcrossLanguages(t *testing.T) {
	cases := []observabilityMatrixCase{
		observabilityCase("unstructured", "observability.unstructured-log", map[string]string{
			"go":         "package sample\n\nimport \"fmt\"\n\nfunc Run() {\n\tfmt.Println(\"failed\")\n}\n",
			"python":     "def run():\n    print('failed')\n",
			"typescript": "export function run() {\n  console.log('failed')\n}\n",
			"javascript": "export function run() {\n  console.log('failed')\n}\n",
			"cpp":        "#include <iostream>\n\nvoid Run() {\n  std::cout << \"failed\";\n}\n",
		}),
		observabilityCase("error-context", "observability.error-without-context", map[string]string{
			"go":         "package sample\n\nfunc Run(err error) { logger.Error(err) }\n",
			"python":     "def run(error):\n    logging.error(error)\n",
			"typescript": "export function run(error: Error) { logger.error(error) }\n",
			"javascript": "export function run(error) { logger.error(error) }\n",
			"cpp":        "#include <iostream>\n\nvoid Run(const std::exception& error) { std::cerr << error.what(); }\n",
		}),
		observabilityCase("sensitive", "observability.sensitive-log-data", map[string]string{
			"go":         "package sample\n\nfunc Run(token string) { logger.Info(\"token\", token) }\n",
			"python":     "def run(password):\n    logging.info('password=%s', password)\n",
			"typescript": "export function run(token: string) { logger.info('token', token) }\n",
			"javascript": "export function run(token) { logger.info('token', token) }\n",
			"cpp":        "#include <cstdio>\n\nvoid Run(const char* password) { printf(\"password=%s\", password); }\n",
		}),
		observabilityCase("cardinality", "observability.high-cardinality-label", map[string]string{
			"go":         "package sample\n\nfunc Run(userID string) { requests.WithLabelValues(userID).Inc() }\n",
			"python":     "def run(user_id):\n    requests.labels(user_id=user_id).inc()\n",
			"typescript": "export function run(request_id: string) { latency.labels({ request_id }).observe(1) }\n",
			"javascript": "export function run(request_id) { latency.labels({ request_id }).observe(1) }\n",
			"cpp":        "void Run(std::string user_id) { latency.labels(user_id).Observe(1); }\n",
		}),
		observabilityCase("critical", "observability.critical-path-uninstrumented", map[string]string{
			"go":         "package sample\n\nfunc CheckoutHandler() {}\n",
			"python":     "def payment_handler():\n    return True\n",
			"typescript": "export function orderConsumer(event: Event) { return event }\n",
			"javascript": "export function orderConsumer(event) { return event }\n",
			"cpp":        "void Payments::Checkout() {}\n",
		}),
		observabilityCase("log-ignore", "observability.log-and-ignore", map[string]string{
			"go":         "package sample\n\nfunc Run(err error) error {\n\tlogger.Error(err)\n\treturn nil\n}\n",
			"python":     "def run(error):\n    logging.error(error)\n    return None\n",
			"typescript": "export function run(error: Error) {\n  logger.error(error)\n  return\n}\n",
			"javascript": "export function run(error) {\n  logger.error(error)\n  return\n}\n",
			"cpp":        "#include <iostream>\n\nbool Run(const std::exception& error) {\n  std::cerr << error.what();\n  return true;\n}\n",
		}),
		observabilityCase("health", "observability.shallow-health-check", map[string]string{
			"go":         "package sample\n\nfunc Healthz() string {\n\tdb.Ping()\n\treturn \"ok\"\n}\n",
			"python":     "def healthz():\n    db.ping()\n    return 'ok'\n",
			"typescript": "export function healthz(db: DB) {\n  db.ping()\n  return StatusOK\n}\n",
			"javascript": "export function healthz(db) {\n  db.ping()\n  return StatusOK\n}\n",
			"cpp":        "std::string Healthz(DB& db) {\n  db.Ping();\n  return \"ok\";\n}\n",
		}),
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for language, source := range tc.sources {
				t.Run(language, func(t *testing.T) {
					dir := t.TempDir()
					writeFile(t, filepath.Join(dir, observabilityMatrixFile(language)), source)

					report, err := codeguard.Run(context.Background(), observabilityConfig("observability-"+tc.name+"-"+language, dir, language))
					if err != nil {
						t.Fatalf("run: %v", err)
					}

					assertFindingRulePresent(t, report, "Observability", tc.ruleID)
				})
			}
		})
	}
}

type observabilityMatrixCase struct {
	name    string
	ruleID  string
	sources map[string]string
}

func observabilityCase(name string, ruleID string, sources map[string]string) observabilityMatrixCase {
	return observabilityMatrixCase{name: name, ruleID: ruleID, sources: sources}
}

func observabilityMatrixFile(language string) string {
	switch language {
	case "go":
		return "handler.go"
	case "python":
		return "handler.py"
	case "typescript":
		return "handler.ts"
	case "javascript":
		return "handler.js"
	default:
		return "handler.cpp"
	}
}
