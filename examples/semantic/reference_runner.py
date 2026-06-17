#!/usr/bin/env python3
"""
Reference semantic runner for CODEGUARD_SEMANTIC_COMMAND.

This script is intentionally conservative:
- it reads the semantic-review request JSON from stdin
- it renders the canonical prompt text from request.prompt plus local evidence
- it can either return {"verdicts": []}, call a local command, or call an
  OpenAI-compatible chat/completions endpoint without changing the prompt assembly

Use it as a scaffold for a real model-backed semantic command.
Set CODEGUARD_SEMANTIC_REFERENCE_PRINT_PROMPT=1 to print the rendered prompt to stderr.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import urllib.error
import urllib.request
from typing import Any

DEFAULT_OPENAI_BASE_URL = "https://api.openai.com/v1"
DEFAULT_OPENAI_MODEL = "gpt-5"


def main() -> int:
    request = json.load(sys.stdin)
    prompt = render_prompt(request)
    if os.getenv("CODEGUARD_SEMANTIC_REFERENCE_PRINT_PROMPT", "").strip().lower() in {
        "1",
        "true",
        "yes",
        "on",
    }:
        sys.stderr.write(prompt)
        if not prompt.endswith("\n"):
            sys.stderr.write("\n")
    json.dump(evaluate(request, prompt), sys.stdout)
    return 0


def evaluate(request: dict[str, Any], prompt: str) -> dict[str, Any]:
    mode = (os.getenv("CODEGUARD_SEMANTIC_REFERENCE_MODE", "scaffold") or "").strip().lower()
    if mode in {"", "scaffold"}:
        return {"verdicts": []}
    if mode == "command":
        return run_local_command(request, prompt)
    if mode == "openai":
        return run_openai_compatible(request, prompt)
    raise SystemExit(f"unsupported CODEGUARD_SEMANTIC_REFERENCE_MODE: {mode}")


def run_local_command(request: dict[str, Any], prompt: str) -> dict[str, Any]:
    command = (os.getenv("CODEGUARD_SEMANTIC_REFERENCE_LOCAL_COMMAND", "") or "").strip()
    if not command:
        raise SystemExit("CODEGUARD_SEMANTIC_REFERENCE_LOCAL_COMMAND is required for command mode")
    payload = json.dumps({"request": request, "prompt_text": prompt})
    completed = subprocess.run(
        command,
        input=payload,
        text=True,
        shell=True,
        capture_output=True,
        check=False,
    )
    if completed.returncode != 0:
        raise SystemExit(
            "semantic reference local command failed: "
            f"{completed.returncode}: {completed.stderr.strip()}"
        )
    return parse_verdict_response(completed.stdout)


def run_openai_compatible(request: dict[str, Any], prompt: str) -> dict[str, Any]:
    base_url = (os.getenv("CODEGUARD_SEMANTIC_REFERENCE_OPENAI_BASE_URL", "") or "").strip()
    if not base_url:
        base_url = DEFAULT_OPENAI_BASE_URL
    model = (os.getenv("CODEGUARD_SEMANTIC_REFERENCE_OPENAI_MODEL", "") or "").strip()
    if not model:
        model = DEFAULT_OPENAI_MODEL
    api_key = (os.getenv("CODEGUARD_SEMANTIC_REFERENCE_OPENAI_API_KEY", "") or "").strip()
    body = {
        "model": model,
        "response_format": {"type": "json_object"},
        "messages": [
            {
                "role": "system",
                "content": (
                    "You are a semantic code reviewer. Return JSON only with the shape "
                    '{"verdicts":[{"rule_id":"...","path":"...","line":1,"level":"warn","message":"..."}]}. '
                    "If the local evidence is insufficient, return an empty verdicts array."
                ),
            },
            {"role": "user", "content": prompt},
        ],
    }
    req = urllib.request.Request(
        base_url.rstrip("/") + "/chat/completions",
        data=json.dumps(body).encode("utf-8"),
        headers={
            "Content-Type": "application/json",
            **({"Authorization": f"Bearer {api_key}"} if api_key else {}),
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req) as resp:
            payload = json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as err:
        detail = err.read().decode("utf-8", errors="replace")
        raise SystemExit(f"semantic reference OpenAI request failed: {err.code}: {detail}") from err
    except urllib.error.URLError as err:
        raise SystemExit(f"semantic reference OpenAI request failed: {err.reason}") from err

    choices = payload.get("choices") or []
    if not choices:
        raise SystemExit("semantic reference OpenAI response contained no choices")
    message = ((choices[0] or {}).get("message") or {}).get("content")
    if isinstance(message, list):
        message = "".join(
            part.get("text", "") for part in message if isinstance(part, dict)
        )
    if not isinstance(message, str) or not message.strip():
        raise SystemExit("semantic reference OpenAI response contained empty content")
    return parse_verdict_response(message)


def parse_verdict_response(raw: str) -> dict[str, Any]:
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError as err:
        raise SystemExit(f"semantic reference backend returned invalid JSON: {err}") from err
    if not isinstance(payload, dict):
        raise SystemExit("semantic reference backend must return a JSON object")
    verdicts = payload.get("verdicts")
    if not isinstance(verdicts, list):
        raise SystemExit('semantic reference backend must return {"verdicts":[...]}')
    return payload


def render_prompt(request: dict[str, Any]) -> str:
    lines: list[str] = []
    prompt = request.get("prompt") or {}

    lines.append("Semantic review request")
    lines.append("")
    append_if_value(lines, "Target", request.get("target_name"))
    append_if_value(lines, "Path", request.get("target_path"))
    append_if_value(lines, "Language", request.get("language"))
    append_if_value(lines, "Base ref", request.get("base_ref"))
    lines.append("")

    overview = prompt.get("overview")
    if overview:
        lines.append("Overview")
        lines.append(overview)
        lines.append("")

    requirements = prompt.get("response_requirements") or []
    if requirements:
        lines.append("Response requirements")
        for item in requirements:
            lines.append(f"- {item}")
        lines.append("")

    checks = request.get("checks") or []
    if checks:
        lines.append("Requested checks")
        for check in checks:
            lines.append(f"- {check.get('rule_id')}: {check.get('title', '')}".rstrip(": "))
            description = check.get("description")
            if description:
                lines.append(f"  Description: {description}")
        lines.append("")

    rule_instructions = prompt.get("rule_instructions") or []
    if rule_instructions:
        lines.append("Rule-specific instructions")
        for rule in rule_instructions:
            lines.append(f"- {rule.get('rule_id')}")
            focus = rule.get("focus")
            if focus:
                lines.append(f"  Focus: {focus}")
            for item in rule.get("consider") or []:
                lines.append(f"  Consider: {item}")
            for item in rule.get("avoid") or []:
                lines.append(f"  Avoid: {item}")
            threshold = rule.get("threshold")
            if threshold:
                lines.append(f"  Threshold: {threshold}")
        lines.append("")

    frameworks = request.get("frameworks") or []
    framework_instructions = prompt.get("framework_instructions") or []
    if frameworks or framework_instructions:
        lines.append("Framework context")
        for item in framework_instructions:
            lines.append(f"- {item.get('name')} {item.get('path')}".strip())
            hints = item.get("hints") or []
            if hints:
                lines.append(f"  Hints: {', '.join(hints)}")
            for advice in item.get("advice") or []:
                lines.append(f"  Advice: {advice}")
        if frameworks and not framework_instructions:
            for item in frameworks:
                lines.append(f"- {item.get('name')} {item.get('path')}".strip())
                hints = item.get("hints") or []
                if hints:
                    lines.append(f"  Hints: {', '.join(hints)}")
                signals = item.get("signals") or []
                if signals:
                    lines.append(f"  Signals: {', '.join(signals)}")
        lines.append("")

    changed_files = request.get("changed_files") or []
    if changed_files:
        lines.append("Changed files")
        for path in changed_files:
            lines.append(f"- {path}")
        lines.append("")

    diff_text = request.get("diff") or ""
    if diff_text:
        lines.append("Diff")
        lines.append("```diff")
        lines.append(diff_text.rstrip("\n"))
        lines.append("```")
        lines.append("")

    append_snapshots(lines, "Source snapshots", request.get("source_files") or [])
    append_snapshots(lines, "Test snapshots", request.get("test_files") or [])
    lines.append('Return JSON only: {"verdicts":[{"rule_id":"...","path":"...","line":1,"level":"warn","message":"..."}]}')
    return "\n".join(lines).rstrip() + "\n"


def append_snapshots(lines: list[str], heading: str, snapshots: list[dict[str, Any]]) -> None:
    if not snapshots:
        return
    lines.append(heading)
    for snapshot in snapshots:
        path = snapshot.get("path", "")
        content = (snapshot.get("content") or "").rstrip("\n")
        lines.append(f"File: {path}")
        lines.append("```")
        lines.append(content)
        lines.append("```")
    lines.append("")


def append_if_value(lines: list[str], label: str, value: Any) -> None:
    if value is None:
        return
    text = str(value).strip()
    if text:
        lines.append(f"{label}: {text}")


if __name__ == "__main__":
    raise SystemExit(main())
