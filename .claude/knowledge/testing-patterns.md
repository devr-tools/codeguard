# Testing Patterns

Testing strategies, test infrastructure quirks, how to run/debug specific test suites, mocking conventions.

- The trust policy (`internal/codeguard/trust`) is a process-global. Tests that exercise command-driven checks or local (127.0.0.1) AI endpoints must enable it. `tests/checks` and `tests/codeguard` each have a `TestMain` (`trust_main_test.go`) that sets `Policy{AllowConfigCommands: true, AllowConfigAIEndpoints: true}`. If you add a new test package that runs config commands or hits a loopback test server, add a similar `TestMain` or the tests will fail with "refusing to run config-supplied command" / "ssrf guard" errors.
- `tests/security` deliberately has NO trust `TestMain`: it verifies the secure *default*. Tests there set/restore the policy locally with `trust.Set(...)` + `t.Cleanup(trust.ResetFromEnv)`. Do not add a package-wide opt-in there.
- All tests live under `tests/` as external (`_test`) packages; there are no white-box tests inside `internal/`/`pkg/`. Internal packages are still importable from `tests/` because they are within the module.
