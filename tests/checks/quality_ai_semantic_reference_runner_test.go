package checks_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSemanticReferenceRunnerPrintsCanonicalPromptAndValidJSON(t *testing.T) {
	root := repoRoot(t)
	scriptPath := filepath.Join(root, "examples", "semantic", "reference_runner.py")
	payload := semanticReferenceRequest(t)

	cmd := exec.Command("python3", scriptPath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CODEGUARD_SEMANTIC_REFERENCE_PRINT_PROMPT=1")
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run reference runner: %v\nstderr:\n%s", err, stderr.String())
	}

	var resp struct {
		Verdicts []any `json:"verdicts"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v\nstdout:\n%s", err, stdout.String())
	}
	if len(resp.Verdicts) != 0 {
		t.Fatalf("verdicts = %#v, want empty scaffold response", resp.Verdicts)
	}

	prompt := stderr.String()
	assertContainsAll(t, prompt,
		"Rule-specific instructions",
		"quality.ai.contract-drift",
		"quality.ai.semantic-test-adequacy",
		"changed params or searchParams handling changes the expected route input contract",
		"tests prove next() chaining",
		"Framework context",
		"Treat next() flow as part of middleware sequencing semantics.",
		"Return JSON only:",
	)
}

func TestSemanticReferenceRunnerCommandModeUsesCanonicalPrompt(t *testing.T) {
	root := repoRoot(t)
	scriptPath := filepath.Join(root, "examples", "semantic", "reference_runner.py")
	backendPath := filepath.Join(t.TempDir(), "semantic-backend.py")
	writeFile(t, backendPath, `#!/usr/bin/env python3
import json
import sys

payload = json.load(sys.stdin)
prompt = payload["prompt_text"]
if "tests prove next() chaining" not in prompt:
    raise SystemExit("missing middleware test guidance")
if "changed params or searchParams handling changes the expected route input contract" not in prompt:
    raise SystemExit("missing nextjs route input guidance")
json.dump({"verdicts":[{"rule_id":"quality.ai.contract-drift","path":"src/auth.ts","line":2,"level":"warn","message":"backend verified prompt contract"}]}, sys.stdout)
`)
	if err := os.Chmod(backendPath, 0o755); err != nil {
		t.Fatalf("chmod backend: %v", err)
	}

	payload := semanticReferenceRequest(t)
	cmd := exec.Command("python3", scriptPath)
	cmd.Dir = root
	cmd.Env = append(
		os.Environ(),
		"CODEGUARD_SEMANTIC_REFERENCE_MODE=command",
		"CODEGUARD_SEMANTIC_REFERENCE_LOCAL_COMMAND=python3 "+backendPath,
	)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run reference runner command mode: %v\nstderr:\n%s", err, stderr.String())
	}

	var resp struct {
		Verdicts []map[string]any `json:"verdicts"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v\nstdout:\n%s", err, stdout.String())
	}
	if len(resp.Verdicts) != 1 || resp.Verdicts[0]["rule_id"] != "quality.ai.contract-drift" {
		t.Fatalf("verdicts = %#v", resp.Verdicts)
	}
}

func TestSemanticReferenceRunnerOpenAICompatibleMode(t *testing.T) {
	root := repoRoot(t)
	scriptPath := filepath.Join(root, "examples", "semantic", "reference_runner.py")
	server := httptest.NewServer(http.HandlerFunc(semanticOpenAIHandler(t)))
	defer server.Close()

	payload := semanticReferenceRequest(t)
	cmd := exec.Command("python3", scriptPath)
	cmd.Dir = root
	cmd.Env = append(
		os.Environ(),
		"CODEGUARD_SEMANTIC_REFERENCE_MODE=openai",
		"CODEGUARD_SEMANTIC_REFERENCE_OPENAI_BASE_URL="+server.URL,
		"CODEGUARD_SEMANTIC_REFERENCE_OPENAI_API_KEY=test-openai-key",
		"CODEGUARD_SEMANTIC_REFERENCE_OPENAI_MODEL=gpt-test-semantic",
	)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run reference runner openai mode: %v\nstderr:\n%s", err, stderr.String())
	}

	var resp struct {
		Verdicts []map[string]any `json:"verdicts"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v\nstdout:\n%s", err, stdout.String())
	}
	if len(resp.Verdicts) != 1 || resp.Verdicts[0]["rule_id"] != "quality.ai.semantic-test-adequacy" {
		t.Fatalf("verdicts = %#v", resp.Verdicts)
	}
}

func semanticReferenceRequest(t *testing.T) []byte {
	t.Helper()
	payload, err := json.Marshal(semanticReferenceRequestBody())
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return payload
}

func semanticReferenceRequestBody() map[string]any {
	return map[string]any{
		"target_name":   "repo",
		"target_path":   ".",
		"language":      "typescript",
		"checks":        semanticReferenceChecks(),
		"frameworks":    semanticReferenceFrameworks(),
		"prompt":        semanticReferencePrompt(),
		"changed_files": []string{"src/auth.ts", "app/users/page.tsx"},
		"diff":          semanticReferenceDiff(),
		"source_files": []map[string]any{
			{
				"path":    "src/auth.ts",
				"content": "export function authMiddleware(req, res, next) {\n  next()\n}\n",
			},
		},
		"test_files": []map[string]any{
			{
				"path":    "src/auth.test.ts",
				"content": "test('auth', () => {})\n",
			},
		},
	}
}

func semanticReferenceChecks() []map[string]any {
	return []map[string]any{
		{
			"rule_id":     "quality.ai.contract-drift",
			"title":       "Silent contract drift",
			"description": "Flag changed functions whose observable behavior appears to drift from the existing contract.",
		},
		{
			"rule_id":     "quality.ai.semantic-test-adequacy",
			"title":       "Tests appear inadequate for changed behavior",
			"description": "Flag changed behavior when nearby tests look too weak or mismatched.",
		},
	}
}

func semanticReferenceFrameworks() []map[string]any {
	return []map[string]any{
		{
			"name":    "express",
			"path":    "src/auth.ts",
			"hints":   []string{"middleware-next-chain", "middleware-order-sensitive"},
			"signals": []string{"express-import"},
		},
		{
			"name":    "nextjs",
			"path":    "app/users/page.tsx",
			"hints":   []string{"route-props-contract", "client-component"},
			"signals": []string{"app-router-component-file", "use-client-directive"},
		},
	}
}

func semanticReferencePrompt() map[string]any {
	return map[string]any{
		"overview": "Review only the changed behavior and nearby tests.",
		"response_requirements": []string{
			"Return JSON only.",
			"If uncertain, omit the verdict rather than guessing.",
		},
		"rule_instructions": []map[string]any{
			{
				"rule_id": "quality.ai.contract-drift",
				"focus":   "Find changed behavior that silently shifts the observable contract.",
				"consider": []string{
					"For Next.js route-segment components, check whether changed params or searchParams handling changes the expected route input contract.",
					"For Express middleware, check whether changed next() flow alters which downstream handlers run.",
				},
			},
			{
				"rule_id": "quality.ai.semantic-test-adequacy",
				"focus":   "Find changed behavior where nearby tests appear too weak.",
				"consider": []string{
					"For React or Next.js components, check whether tests cover changed prop combinations.",
					"For Express middleware, check whether tests prove next() chaining.",
				},
			},
		},
		"framework_instructions": []map[string]any{
			{
				"name":   "express",
				"path":   "src/auth.ts",
				"hints":  []string{"middleware-next-chain"},
				"advice": []string{"Treat next() flow as part of middleware sequencing semantics."},
			},
		},
	}
}

func semanticReferenceDiff() string {
	return strings.Join([]string{
		"diff --git a/src/auth.ts b/src/auth.ts",
		"--- a/src/auth.ts",
		"+++ b/src/auth.ts",
		"@@ -1,3 +1,4 @@",
		" export function authMiddleware(req, res, next) {",
		"+  res.locals.user = req.headers.authorization",
		"   next()",
		" }",
	}, "\n")
}
