package checks

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	govulncheckrunner "github.com/devr-tools/codeguard/internal/codeguard/runner/govulncheck"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func govulncheckCallback(sc runnersupport.Context) func(context.Context, string, string) ([]core.Finding, error) {
	return func(ctx context.Context, dir, command string) ([]core.Finding, error) {
		return govulncheckrunner.Run(ctx, dir, command, sc)
	}
}
