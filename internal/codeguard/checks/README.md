# Checks

Built-in checks live under `internal/codeguard/checks/` and are split by policy category:

- `quality/`
- `design/`
- `security/`
- `prompts/`
- `ci/`

`support/` contains the shared adapter surface each check package uses to talk to the runner without duplicating scan orchestration.

This is the only active implementation tree for built-in checks. As support for additional languages is added, language-specific rules should stay under the relevant category package instead of creating a second top-level implementation path.
