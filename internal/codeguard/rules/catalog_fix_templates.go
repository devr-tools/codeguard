package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// fixTemplates holds concrete, agent-actionable fix instructions per rule:
// a short imperative description plus a before/after snippet where one makes
// sense. They surface through explain --format=agent and the MCP explain tool.
var fixTemplates = map[string]string{
	"quality.gofmt":                         "Run gofmt -w on the file and commit the formatted result.\n\nBefore:\nfunc main(){fmt.Println(\"hi\")}\n\nAfter:\nfunc main() {\n\tfmt.Println(\"hi\")\n}",
	"quality.ai.swallowed-error":            "Handle the error explicitly instead of discarding it.\n\nBefore:\nresult, _ := doWork()\n\nAfter:\nresult, err := doWork()\nif err != nil {\n\treturn fmt.Errorf(\"do work: %w\", err)\n}",
	"quality.ai.hallucinated-import":        "Replace the unresolved import with a dependency that exists in the repository, or add the dependency to the module manifest intentionally.\n\nBefore:\nimport { fetchJson } from \"super-fetch-utils\"; // not in package.json\n\nAfter:\n// either: npm install super-fetch-utils\n// or import the local equivalent that already exists:\nimport { fetchJson } from \"./lib/fetch-json\";",
	"quality.ai.narrative-comment":          "Delete comments that narrate the adjacent code, or rewrite them to capture intent, constraints, or tradeoffs.\n\nBefore:\n// loop over the users and print each one\nfor _, user := range users {\n\tfmt.Println(user)\n}\n\nAfter:\n// emit one line per user so the audit job can diff snapshots\nfor _, user := range users {\n\tfmt.Println(user)\n}",
	"quality.ai.dead-code":                  "Delete the unreachable constant-condition branch or replace the placeholder with a real runtime check.\n\nBefore:\nif (false) {\n  runLegacyMigration();\n}\n\nAfter:\nif (config.legacyMigrationEnabled) {\n  runLegacyMigration();\n}\n// or delete the branch and its dead callee entirely",
	"quality.ai.over-mocked-test":           "Exercise the real unit boundary and assert on observable behavior instead of mock wiring.\n\nBefore:\nmockRepo.On(\"Save\", mock.Anything).Return(nil)\nsvc.Create(user)\nmockRepo.AssertExpectations(t)\n\nAfter:\nrepo := newInMemoryRepo()\nsvc := NewService(repo)\nsvc.Create(user)\nif got := len(repo.All()); got != 1 {\n\tt.Fatalf(\"saved users = %d, want 1\", got)\n}",
	"quality.max-function-lines":            "Extract cohesive steps into named helpers until the function fits the configured limit.\n\nBefore:\nfunc handle(req Request) (Response, error) {\n\t// dozens of lines: validate, transform, persist, render\n}\n\nAfter:\nfunc handle(req Request) (Response, error) {\n\tinput, err := validate(req)\n\tif err != nil {\n\t\treturn Response{}, err\n\t}\n\trecord := transform(input)\n\tif err := persist(record); err != nil {\n\t\treturn Response{}, err\n\t}\n\treturn render(record), nil\n}",
	"quality.cyclomatic-complexity":         "Flatten branching with early returns, or replace branch ladders with table-driven dispatch.\n\nBefore:\nif ok {\n\tif valid {\n\t\tif ready {\n\t\t\tprocess()\n\t\t}\n\t}\n}\n\nAfter:\nif !ok || !valid || !ready {\n\treturn\n}\nprocess()",
	"quality.typescript.explicit-any":       "Replace any with a precise type, a generic constraint, or unknown plus narrowing.\n\nBefore:\nfunction parse(input: any): any {\n  return JSON.parse(input);\n}\n\nAfter:\nfunction parse<T>(input: string): T {\n  return JSON.parse(input) as T;\n}",
	"quality.typescript.ts-ignore":          "Fix the underlying type error and delete the @ts-ignore suppression.\n\nBefore:\n// @ts-ignore\nconst id = user.id;\n\nAfter:\nif (user === undefined) {\n  throw new Error(\"user is required\");\n}\nconst id = user.id;",
	"quality.typescript.debugger-statement": "Remove the committed debugger statement; use tests or structured logging instead.\n\nBefore:\nfunction onSubmit(data: FormData) {\n  debugger;\n  send(data);\n}\n\nAfter:\nfunction onSubmit(data: FormData) {\n  send(data);\n}",
	"quality.typescript.non-null-assertion": "Prove nullability with a guard instead of asserting it away with !.\n\nBefore:\nconst name = user!.name;\n\nAfter:\nif (user === null) {\n  throw new Error(\"user is required\");\n}\nconst name = user.name;",
	"quality.javascript.explicit-any":       "Replace any with a precise type, a generic constraint, or unknown plus narrowing.\n\nBefore:\n/** @param {any} input */\nfunction parse(input) {\n  return JSON.parse(input);\n}\n\nAfter:\n/** @param {string} input @returns {Config} */\nfunction parse(input) {\n  return JSON.parse(input);\n}",
	"quality.javascript.ts-ignore":          "Fix the underlying type error and delete the @ts-ignore suppression.\n\nBefore:\n// @ts-ignore\nconst id = user.id;\n\nAfter:\nif (user === undefined) {\n  throw new Error(\"user is required\");\n}\nconst id = user.id;",
	"quality.javascript.debugger-statement": "Remove the committed debugger statement; use tests or structured logging instead.\n\nBefore:\nfunction onSubmit(data) {\n  debugger;\n  send(data);\n}\n\nAfter:\nfunction onSubmit(data) {\n  send(data);\n}",
	"quality.javascript.non-null-assertion": "Prove nullability with a guard instead of asserting it away with !.\n\nBefore:\nconst name = user!.name;\n\nAfter:\nif (user === null) {\n  throw new Error(\"user is required\");\n}\nconst name = user.name;",
	"prompts.secret-interpolation":          "Remove secret placeholders from prompt assets and inject credentials outside the prompt text.\n\nBefore:\nUse the API key ${OPENAI_API_KEY} when calling the downstream service.\n\nAfter:\nCall the downstream service through the pre-authenticated client. Never place credentials in prompt text.",
	"prompts.agent-standing-permissions":    "Scope agent tool permissions to the minimum required commands, paths, and hosts.\n\nBefore:\n{\n  \"permissions\": { \"allow\": [\"Bash(*)\"] }\n}\n\nAfter:\n{\n  \"permissions\": { \"allow\": [\"Bash(go build ./...)\", \"Bash(go test ./...)\"] }\n}",
	"prompts.mcp-config-risk":               "Pin MCP servers to fixed binaries and replace wildcard tool allowlists with named tools.\n\nBefore:\n{\n  \"command\": \"sh\",\n  \"args\": [\"-c\", \"npx some-mcp-server\"],\n  \"alwaysAllow\": [\"*\"]\n}\n\nAfter:\n{\n  \"command\": \"npx\",\n  \"args\": [\"some-mcp-server\"],\n  \"alwaysAllow\": [\"list_issues\", \"read_file\"]\n}",
	"ci.test-without-assertion":             "Add a real assertion or explicit failure path so the test verifies observable behavior.\n\nBefore:\nfunc TestProcess(t *testing.T) {\n\tProcess(input)\n}\n\nAfter:\nfunc TestProcess(t *testing.T) {\n\tgot := Process(input)\n\tif got != want {\n\t\tt.Fatalf(\"Process() = %v, want %v\", got, want)\n\t}\n}",
}

func applyFixTemplate(meta core.RuleMetadata) core.RuleMetadata {
	if meta.FixTemplate != "" {
		return meta
	}
	if template, ok := fixTemplates[meta.ID]; ok {
		meta.FixTemplate = template
	}
	return meta
}
