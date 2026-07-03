# codeguard

Repository checks for code quality, design boundaries, security, CI/CD hygiene,
and AI prompt governance — a standalone Go CLI.

```bash
npm install -g @devr-tools/codeguard
codeguard --version
```

This package is a thin launcher. The actual binary is delivered through a
platform-specific optional dependency (for example
`@devr-tools/codeguard-darwin-arm64`) that npm selects automatically for your OS
and CPU — there is no download step at install time.

Full documentation: https://github.com/devr-tools/codeguard
