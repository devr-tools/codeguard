package checks

import (
	"context"

	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	govulncheckrunner "github.com/devr-tools/codeguard/internal/codeguard/runner/govulncheck"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func govulncheckCallback(sc runnersupport.Context) func(context.Context, string, string) checkSupport.GovulncheckResult {
	return func(ctx context.Context, dir, command string) checkSupport.GovulncheckResult {
		findings, diagnostics := govulncheckrunner.RunWorkspace(ctx, dir, command, sc)
		return checkSupport.GovulncheckResult{Findings: findings, Diagnostics: diagnostics}
	}
}
