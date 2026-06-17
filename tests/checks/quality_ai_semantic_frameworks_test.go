package checks_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualitySemanticChecksIncludeExpressFrameworkContext(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "routes.ts"), `import express from "express"

const router = express.Router()

export function mountRoutes(app: express.Express) {
	app.use("/api", router)
}
`)
	diff := stringsJoin(
		"diff --git a/src/routes.ts b/src/routes.ts",
		"--- a/src/routes.ts",
		"+++ b/src/routes.ts",
		"@@ -2,5 +2,8 @@",
		" import express from \"express\"",
		" ",
		" const router = express.Router()",
		" ",
		" export function mountRoutes(app: express.Express) {",
		"+\trouter.get(\"/users/:id\", async (_req, res) => {",
		"+\t\tres.status(500).json({ error: \"failed\" })",
		"+\t})",
		" \tapp.use(\"/api\", router)",
		" }",
	)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	requestPath := filepath.Join(dir, "semantic-request.json")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticCaptureScript(counterPath, requestPath, `{"verdicts":[]}`))

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	if _, err := codeguard.RunPatch(context.Background(), qualityAISemanticConfig(dir, "quality-ai-semantic-express"), diff); err != nil {
		t.Fatalf("run patch: %v", err)
	}

	var req struct {
		Frameworks []struct {
			Name    string   `json:"name"`
			Path    string   `json:"path"`
			Signals []string `json:"signals"`
			Hints   []string `json:"hints"`
		} `json:"frameworks"`
		Prompt semanticPromptTemplate `json:"prompt"`
	}
	data, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if len(req.Frameworks) != 1 {
		t.Fatalf("frameworks = %#v, want 1 express framework entry", req.Frameworks)
	}
	if req.Frameworks[0].Name != "express" || req.Frameworks[0].Path != "src/routes.ts" {
		t.Fatalf("framework entry = %#v, want express src/routes.ts", req.Frameworks[0])
	}
	assertStringSliceContainsAll(t, req.Frameworks[0].Signals, "express-import", "express-router", "http-route-handler")
	assertStringSliceContainsAll(t, req.Frameworks[0].Hints, "middleware-order-sensitive", "response-side-effects")
	assertRulePromptContainsAll(t, req.Prompt, "quality.ai.contract-drift", "middleware ordering alters which downstream handlers run")
	assertRulePromptContainsAll(t, req.Prompt, "quality.ai.semantic-test-adequacy", "tests prove next() chaining")
}

func TestQualitySemanticChecksIncludeNextJSFrameworkContext(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "api", "users", "route.ts"), `import { NextRequest, NextResponse } from "next/server"

export async function POST(request: NextRequest) {
	return NextResponse.json({ ok: true }, { status: 201 })
}
`)
	diff := stringsJoin(
		"diff --git a/app/api/users/route.ts b/app/api/users/route.ts",
		"--- a/app/api/users/route.ts",
		"+++ b/app/api/users/route.ts",
		"@@ -1,4 +1,5 @@",
		" import { NextRequest, NextResponse } from \"next/server\"",
		" ",
		" export async function POST(request: NextRequest) {",
		"+\tconst body = await request.json()",
		"-\treturn NextResponse.json({ ok: true }, { status: 201 })",
		"+\treturn NextResponse.json({ id: body.id }, { status: 201 })",
		" }",
	)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	requestPath := filepath.Join(dir, "semantic-request.json")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticCaptureScript(counterPath, requestPath, `{"verdicts":[]}`))

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	if _, err := codeguard.RunPatch(context.Background(), qualityAISemanticConfig(dir, "quality-ai-semantic-nextjs"), diff); err != nil {
		t.Fatalf("run patch: %v", err)
	}

	var req struct {
		Frameworks []struct {
			Name    string   `json:"name"`
			Path    string   `json:"path"`
			Signals []string `json:"signals"`
			Hints   []string `json:"hints"`
		} `json:"frameworks"`
		Prompt semanticPromptTemplate `json:"prompt"`
	}
	data, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if len(req.Frameworks) != 1 {
		t.Fatalf("frameworks = %#v, want 1 nextjs framework entry", req.Frameworks)
	}
	if req.Frameworks[0].Name != "nextjs" || req.Frameworks[0].Path != "app/api/users/route.ts" {
		t.Fatalf("framework entry = %#v, want nextjs app/api/users/route.ts", req.Frameworks[0])
	}
	assertStringSliceContainsAll(t, req.Frameworks[0].Signals, "app-router-route-file", "next-request-response", "next-server-import", "route-handler-export")
	assertStringSliceContainsAll(t, req.Frameworks[0].Hints, "route-handler-contract", "async-data-contract")
	assertRulePromptContainsAll(t, req.Prompt, "quality.ai.contract-drift", "changed request parsing, status codes, or response payloads shift the handler contract")
	assertRulePromptContainsAll(t, req.Prompt, "quality.ai.semantic-test-adequacy", "tests cover changed request shapes, status codes, and response bodies")
}

func TestQualitySemanticChecksIncludeExpressMiddlewareHints(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "auth.ts"), `import express, { NextFunction, Request, Response } from "express"

export function authMiddleware(req: Request, res: Response, next: NextFunction) {
	if (!req.headers.authorization) {
		res.status(401).json({ error: "missing auth" })
		return
	}
	next()
}
`)
	diff := stringsJoin(
		"diff --git a/src/auth.ts b/src/auth.ts",
		"--- a/src/auth.ts",
		"+++ b/src/auth.ts",
		"@@ -2,7 +2,8 @@",
		" ",
		" export function authMiddleware(req: Request, res: Response, next: NextFunction) {",
		" \tif (!req.headers.authorization) {",
		" \t\tres.status(401).json({ error: \"missing auth\" })",
		" \t\treturn",
		" \t}",
		"+\tres.locals.user = req.headers.authorization",
		" \tnext()",
		" }",
	)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	requestPath := filepath.Join(dir, "semantic-request.json")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticCaptureScript(counterPath, requestPath, `{"verdicts":[]}`))

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	if _, err := codeguard.RunPatch(context.Background(), qualityAISemanticConfig(dir, "quality-ai-semantic-express-middleware"), diff); err != nil {
		t.Fatalf("run patch: %v", err)
	}

	frameworks := readSemanticFrameworks(t, requestPath)
	entry := requireFrameworkEntry(t, frameworks, "express", "src/auth.ts")
	assertStringSliceContainsAll(t, entry.Signals, "express-import")
	assertStringSliceContainsAll(t, entry.Hints, "middleware-next-chain", "middleware-order-sensitive", "request-derived-contract", "response-side-effects")
	prompt := readSemanticPrompt(t, requestPath)
	assertRulePromptContainsAll(t, prompt, "quality.ai.contract-drift", "next() flow", "request state they receive")
	assertRulePromptContainsAll(t, prompt, "quality.ai.semantic-test-adequacy", "tests prove next() chaining", "res.locals or request mutation")
}

func TestQualitySemanticChecksIncludeReactComponentFrameworkContext(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "components", "UserCard.tsx"), `import { useState } from "react"

interface Props {
	userID: string
}

export function UserCard({ userID }: Props) {
	const [expanded, setExpanded] = useState(false)
	return <button onClick={() => setExpanded(!expanded)}>{userID}</button>
}
`)
	diff := stringsJoin(
		"diff --git a/src/components/UserCard.tsx b/src/components/UserCard.tsx",
		"--- a/src/components/UserCard.tsx",
		"+++ b/src/components/UserCard.tsx",
		"@@ -5,5 +5,5 @@",
		" ",
		" export function UserCard({ userID }: Props) {",
		"-\tconst [expanded, setExpanded] = useState(false)",
		"+\tconst [expanded, setExpanded] = useState(true)",
		" \treturn <button onClick={() => setExpanded(!expanded)}>{userID}</button>",
		" }",
	)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	requestPath := filepath.Join(dir, "semantic-request.json")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticCaptureScript(counterPath, requestPath, `{"verdicts":[]}`))

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	if _, err := codeguard.RunPatch(context.Background(), qualityAISemanticConfig(dir, "quality-ai-semantic-react-component"), diff); err != nil {
		t.Fatalf("run patch: %v", err)
	}

	frameworks := readSemanticFrameworks(t, requestPath)
	entry := requireFrameworkEntry(t, frameworks, "react", "src/components/UserCard.tsx")
	assertStringSliceContainsAll(t, entry.Signals, "jsx-component", "component-export", "react-hooks")
	assertStringSliceContainsAll(t, entry.Hints, "component-props-contract", "stateful-component")
	prompt := readSemanticPrompt(t, requestPath)
	assertRulePromptContainsAll(t, prompt, "quality.ai.contract-drift", "changed props shape, required props, or children expectations")
	assertRulePromptContainsAll(t, prompt, "quality.ai.semantic-test-adequacy", "tests cover changed prop combinations", "changed interaction or state transition")
}

func TestQualitySemanticChecksIncludeNextJSComponentFrameworkContext(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "users", "page.tsx"), `"use client"

import { useState } from "react"

type Props = {
	searchParams: {
		q?: string
	}
}

export default function UsersPage({ searchParams }: Props) {
	const [query, setQuery] = useState(searchParams.q ?? "")
	return <main><button onClick={() => setQuery("")}>{query}</button></main>
}
`)
	diff := stringsJoin(
		"diff --git a/app/users/page.tsx b/app/users/page.tsx",
		"--- a/app/users/page.tsx",
		"+++ b/app/users/page.tsx",
		"@@ -9,5 +9,5 @@",
		" ",
		" export default function UsersPage({ searchParams }: Props) {",
		"-\tconst [query, setQuery] = useState(searchParams.q ?? \"\")",
		"+\tconst [query, setQuery] = useState((searchParams.q ?? \"\").trim())",
		" \treturn <main><button onClick={() => setQuery(\"\")}>{query}</button></main>",
		" }",
	)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	requestPath := filepath.Join(dir, "semantic-request.json")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticCaptureScript(counterPath, requestPath, `{"verdicts":[]}`))

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	if _, err := codeguard.RunPatch(context.Background(), qualityAISemanticConfig(dir, "quality-ai-semantic-nextjs-component"), diff); err != nil {
		t.Fatalf("run patch: %v", err)
	}

	frameworks := readSemanticFrameworks(t, requestPath)
	nextEntry := requireFrameworkEntry(t, frameworks, "nextjs", "app/users/page.tsx")
	assertStringSliceContainsAll(t, nextEntry.Signals, "app-router-component-file", "use-client-directive")
	assertStringSliceContainsAll(t, nextEntry.Hints, "route-segment-component", "client-component", "route-props-contract")

	reactEntry := requireFrameworkEntry(t, frameworks, "react", "app/users/page.tsx")
	assertStringSliceContainsAll(t, reactEntry.Signals, "jsx-component", "component-export", "react-hooks", "use-client-directive")
	assertStringSliceContainsAll(t, reactEntry.Hints, "component-props-contract", "stateful-component", "client-component")
	prompt := readSemanticPrompt(t, requestPath)
	assertRulePromptContainsAll(t, prompt, "quality.ai.contract-drift", "params or searchParams handling changes the expected route input contract")
	assertRulePromptContainsAll(t, prompt, "quality.ai.semantic-test-adequacy", "tests cover changed params or searchParams inputs")
}
