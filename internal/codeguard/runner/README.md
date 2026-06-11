# Runner

The runner package is organized into focused subdirectories instead of a flat set of `runner_*.go` files:

- `checks/` for built-in section wiring
- `custom/` for custom-rule execution
- `govulncheck/` for external vuln scanning
- `support/` for shared runner helpers and scan context

`runner.go` stays as the small public facade for constructing and executing scans.
